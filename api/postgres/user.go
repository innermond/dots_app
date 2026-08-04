package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/innermond/dots"
	"github.com/segmentio/ksuid"
)

type UserService struct {
	db *DB
}

var userService *UserService

func NewUserService(db *DB) *UserService {
	if userService == nil {
		userService = &UserService{db: nil}
	}
	// old db may be closed
	userService.db = db

	return userService
}

func (s *UserService) FindUserByID(ctx context.Context, id ksuid.KSUID) (*dots.User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	u, err := findUserByID(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	err = attachUserAuths(ctx, tx, u)
	if err != nil {
		return u, err
	}

	return u, nil
}

func (s *UserService) CreateUser(ctx context.Context, u *dots.User) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = createUser(ctx, tx, u); err != nil {
		return err
	}
	err = attachUserAuths(ctx, tx, u)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *UserService) FindUser(ctx context.Context, filter dots.UserFilter) ([]*dots.User, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	return findUser(ctx, tx, filter)
}

func (s *UserService) UpdateUser(ctx context.Context, id ksuid.KSUID, upd dots.UserUpdate) (*dots.User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	user, err := updateUser(ctx, tx, id, &upd)
	if err != nil {
		return user, err
	}

	err = attachUserAuths(ctx, tx, user)
	if err != nil {
		return user, err
	}

	err = tx.Commit()
	if err != nil {
		return user, err
	}

	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id ksuid.KSUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = deleteUser(ctx, tx, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func createUser(ctx context.Context, tx *Tx, u *dots.User) error {
	if err := u.ValidateCreate(); err != nil {
		return err
	}

	var email *string
	if u.Email != "" {
		email = &u.Email
	}

	apiKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, apiKey); err != nil {
		return err
	}
	u.ApiKey = hex.EncodeToString(apiKey)

	err := tx.QueryRowContext(
		ctx, `
		INSERT INTO core."user" (
			name,
			email,
			api_key,
			powers,
			created_at,
			updated_at
		)
		values ($1, $2, $3, $4, $5, $6) returning id
	`,
		u.Name, email, u.ApiKey, u.Powers, tx.now, tx.now,
	).Scan(&u.ID)
	if err != nil {
		return err
	}

	u.CreatedAt = tx.now
	u.UpdatedAt = tx.now

	return nil
}

func updateUser(ctx context.Context, tx *Tx, id ksuid.KSUID, updata *dots.UserUpdate) (*dots.User, error) {
	uu, _, err := findUser(ctx, tx, dots.UserFilter{ID: &id, Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("postgres.user: cannot retrieve user %w", err)
	}
	if len(uu) == 0 {
		return nil, dots.Errorf(dots.ENOTFOUND, "user not found")
	}
	u := uu[0]

	set, args := []string{}, []interface{}{}
	if v := updata.Name; v != nil {
		u.Name = *v
		set, args = append(set, "name = ?"), append(args, *v)
	}
	if v := updata.Email; v != nil {
		u.Email = *v
		set, args = append(set, "email = ?"), append(args, *v)
	}
	u.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	set, args = append(set, "updated_at = ?"), append(args, u.UpdatedAt)

	// At inx 0 we may have a "?" that results in $0 that is not a valid placeholder
	for inx, v := range set {
		v = strings.Replace(v, "?", fmt.Sprintf("$%d", inx+1), 1)
		set[inx] = v
	}
	args = append(args, id)

	sqlstr := `
		update core."user"
		set ` + strings.Join(set, ", ") + `
		where	id = ` + fmt.Sprintf("$%d", len(args))

	_, err = tx.ExecContext(ctx, sqlstr, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.user: cannot update %w", err)
	}

	return u, nil
}

func findUser(ctx context.Context, tx *Tx, filter dots.UserFilter) (_ []*dots.User, n int, err error) {
	where, args := []string{"1 = 1"}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}
	if v := filter.Email; v != nil {
		where, args = append(where, "email = ?"), append(args, *v)
	}
	if v := filter.ApiKey; v != nil {
		where, args = append(where, "api_key = ?"), append(args, *v)
	}
	for inx, v := range where {
		if !strings.Contains(v, "?") {
			continue
		}
		v = strings.Replace(v, "?", fmt.Sprintf("$%d", inx), 1)
		where[inx] = v
	}

	sqlstr := fmt.Sprintf(`
	select
		id, name, email, api_key,
    array_to_json(powers),
		created_at, updated_at,
		count(*) over()
	from core."user" u
	where	%s %s`,
		strings.Join(where, " and "),
		formatLimitOffset(filter.Limit, filter.Offset),
	)

	rows, err := tx.QueryContext(ctx, sqlstr, args...)
	if err == sql.ErrNoRows {
		return nil, 0, dots.Errorf(dots.ENOTFOUND, "user not found")
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []*dots.User{}
	for rows.Next() {
		var u dots.User
		var pp string
		var email sql.NullString
		var createdAt sql.NullTime
		var updatedAt sql.NullTime

		err := rows.Scan(
			&u.ID,
			&u.Name,
			&email,
			&u.ApiKey,
			&pp,
			&createdAt,
			&updatedAt,
			&n,
		)
		if err != nil {
			return nil, 0, err
		}

		var powers []dots.Power
		err = json.Unmarshal([]byte(pp), &powers)
		if err != nil {
			return nil, n, err
		}
		u.Powers = powers
		if email.Valid {
			u.Email = email.String
		}
		u.CreatedAt = timeRFC3339(createdAt)
		u.UpdatedAt = timeRFC3339(updatedAt)

		users = append(users, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, n, nil
}

func findUserByID(ctx context.Context, tx *Tx, id ksuid.KSUID) (*dots.User, error) {
	uu, _, err := findUser(ctx, tx, dots.UserFilter{ID: &id, Limit: 1})
	if err != nil {
		return nil, err
	} else if len(uu) == 0 {
		return nil, fmt.Errorf("postgres.user: user not found")
	}
	return uu[0], nil
}

func deleteUser(ctx context.Context, tx *Tx, id ksuid.KSUID) error {
	uu, _, err := findUser(ctx, tx, dots.UserFilter{ID: &id})
	if err != nil {
		return fmt.Errorf("postgres.user: cannot find user: %w", err)
	}
	if len(uu) == 0 {
		return dots.Errorf(dots.ENOTFOUND, "user not found")
	}

	_, err = tx.ExecContext(ctx, `delete from core."user" where id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.user: cannot delete user: %w", err)
	}

	return nil
}

func attachUserAuths(ctx context.Context, tx *Tx, u *dots.User) (err error) {
	u.Auths, _, err = findAuth(ctx, tx, dots.AuthFilter{UserID: &u.ID})
	if err != nil {
		return err
	}
	return nil
}
