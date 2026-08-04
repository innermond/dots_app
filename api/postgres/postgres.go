package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/innermond/dots"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/segmentio/ksuid"
)

type DB struct {
	db  *sql.DB
	DSN string

	ctx    context.Context
	cancel func()

	Now func() time.Time
}

func NewDB(dsn string) *DB {
	db := &DB{
		DSN: dsn,
		Now: time.Now,
	}
	db.ctx, db.cancel = context.WithCancel(context.Background())

	return db
}

func (db *DB) Open() (err error) {
	if db.DSN == "" {
		return errors.New("dns required")
	}

	db.db, err = sql.Open("pgx", db.DSN)
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) Close() error {
	db.cancel()
	if db.db != nil {
		return db.db.Close()
	}
	return nil
}

type Tx struct {
	*sql.Tx
	db  *DB
	now time.Time
}

func (tx *Tx) RollbackOrCommit(err error) {
	switch err {
	case nil:
		tx.Commit()
	default:
		tx.Rollback()
	}
}

func (tx *Tx) getUserIDSetting(ctx context.Context) (*ksuid.KSUID, error) {
	sqlstr := `select get_tenent()::ksuid;`
	var uidSetting *ksuid.KSUID
	err := tx.QueryRowContext(ctx, sqlstr).Scan(&uidSetting)
	if err != nil {
		return nil, err
	}
	if uidSetting == nil {
		return nil, errors.New("app.uid setting not found")
	}
	return uidSetting, nil
}

func (tx *Tx) setUserIDPerConnection(ctx context.Context) error {
	u := dots.UserFromContext(ctx)
	if u.ID.IsNil() {
		return errors.New("user expected to be found")
	}
	_, err := tx.ExecContext(ctx, "SELECT set_config('app.uid', $1::core.ksuid, true)", u.ID)
	if err != nil {
		return err
	}
	fmt.Printf("set uid per connection %v\n:", u.ID.String())
	return nil
}

func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &Tx{
		Tx:  tx,
		db:  db,
		now: db.Now().UTC().Truncate(time.Second),
	}, nil
}

func formatLimitOffset(limit, offset int) string {
	if limit > 0 && offset > 0 {
		return fmt.Sprintf("limit %d offset %d", limit, offset)
	} else if limit > 0 {
		return fmt.Sprintf("limit %d", limit)
	} else if offset > 0 {
		return fmt.Sprintf("offset %d", offset)
	}
	return ""
}

func timeRFC3339(val sql.NullTime) time.Time {
	if val.Valid {
		loc, err := time.LoadLocation("UTC")
		if err != nil {
			return (*sql.NullTime)(nil).Time
		}
		vs := val.Time.In(loc).Format(time.RFC3339)
		v, err := time.Parse(time.RFC3339, vs)
		if err != nil {
			return (*sql.NullTime)(nil).Time
		}
		return v
	}
	return (*sql.NullTime)(nil).Time
}

// where and args must be constructed togheter to be sync'ed
func replaceQuestionMark(where []string, args []interface{}) {
	args_inx := 0
	// PostgreSQL uses numbered placeholders starting from $1
	for i, v := range where {
		// only question mark are of interest
		if !strings.Contains(v, "?") {
			continue
		}
		// syncing with args
		if args_inx >= len(args) {
			// TODO return error here
			// number of question marks are not same as coresponding args
			// so this hopefully will result in an error into caller
			return
		}

		// we have value in args
		v = strings.Replace(v, "?", fmt.Sprintf("$%d", i+1), 1)
		where[i] = v
		// move index
		args_inx++
	}
}

func aprox(v float64, numberDecimals int) float64 {
	num := math.Pow(10, float64(numberDecimals))
	rounded := math.Round(v*num) / num
	return rounded
}

func perr(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return dots.Errorf(dots.ENOTFOUND, "%s no records", err)
	}

	var perr *pgconn.PgError
	if !errors.As(err, &perr) {
		return err
	}

	switch perr.Code {
	case "23505":
		return dots.Errorf(dots.EINVALID, "duplicate: %v", perr.ConstraintName)
	case "23502":
		return dots.Errorf(dots.EINVALID, "missing value: %v", perr.ColumnName)
	default:
		return errors.New(perr.Message)
	}
}
