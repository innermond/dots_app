package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/innermond/dots"
	"github.com/segmentio/ksuid"
)

type EntryService struct {
	db *DB
}

func NewEntryService(db *DB) *EntryService {
	return &EntryService{db: db}
}

func (s *EntryService) CreateEntry(ctx context.Context, e *dots.Entry) error {
	if err := e.Validate(); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if canerr := dots.CanCreateOwn(ctx); canerr != nil {
		return canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return err
	}

	if err := createEntry(ctx, tx, e); err != nil {
		err = fmt.Errorf("create entry: %w", err)
		return perr(err)
	}

	tx.Commit()

	return nil
}

func (s *EntryService) FindEntry(ctx context.Context, filter dots.EntryFilter) ([]*dots.Entry, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	if canerr := dots.CanReadOwn(ctx); canerr != nil {
		return nil, 0, canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return nil, 0, err
	}

	// need company ID that belong to user
	if filter.CompanyID == nil {
		return nil, 0, dots.Errorf(dots.EINVALID, "missing company")
	}

	return findEntry(ctx, tx, filter)
}

func (s *EntryService) UpdateEntry(ctx context.Context, id int, upd dots.EntryUpdate) (*dots.Entry, error) {
	// TODO valiate?
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if canerr := dots.CanWriteOwn(ctx); canerr != nil {
		return nil, canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return nil, err
	}

	e, err := updateEntry(ctx, tx, id, upd)
	if err != nil {
		return nil, err
	}

	tx.Commit()

	return e, nil
}

func (s *EntryService) DeleteEntry(ctx context.Context, id int, filter dots.EntryDelete) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if canerr := dots.CanDeleteOwn(ctx); canerr != nil {
		return 0, canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return 0, err
	}

	var n int

	if filter.Hard {
		n, err = deleteEntryPermanently(ctx, tx, id)
	} else {
		n, err = deleteEntry(ctx, tx, id, filter.Resurect)
	}
	if err != nil {
		return n, err
	}

	tx.Commit()

	return n, err
}

func createEntry(ctx context.Context, tx *Tx, e *dots.Entry) error {
	// fk checks only the remove row's existence in table
	// check if remote rows has not been deleted (enforced by the view)
	// and has same tid (enforced by row level security)
	// in short we check if company and entry type belongs to user
	check := `
with data_entry as (
  select
    ((select id is not null from company where id = $3) and
     (select id is not null from entry_type where id = $1)) as ok
)`
	sqlstr := check + `
insert into entry (entry_type_id, quantity, company_id, date_added)
select $1, $2, $3, date_trunc('minute', now())::timestamptz from data_entry
where data_entry.ok = true -- apply check here
returning id, date_added;
		`

	var (
		id         int
		date_added time.Time
	)
	err := tx.QueryRowContext(
		ctx,
		sqlstr,
		e.EntryTypeID, e.Quantity, e.CompanyID,
	).Scan(&id, &date_added)
	if err != nil {
		// no rows are returned when insertion fail due to check
		if err == sql.ErrNoRows {
			return errors.New("company or entry type cannot be part of a new entry")
		}
		return err
	}

	// update fields of entry
	e.ID = &id
	e.DateAdded = date_added

	return nil
}

func updateEntry(ctx context.Context, tx *Tx, id int, updata dots.EntryUpdate) (*dots.Entry, error) {
	ee, _, err := findEntry(ctx, tx, dots.EntryFilter{ID: &id, Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("postgres.entry: cannot retrieve entry %w", err)
	}
	if len(ee) == 0 {
		return nil, dots.Errorf(dots.ENOTFOUND, "entry not found")
	}
	e := ee[0]

	set, args := []string{}, []interface{}{}
	checks := []string{}
	if v := updata.EntryTypeID; v != nil {
		e.EntryTypeID = v
		set, args = append(set, "entry_type_id = ?"), append(args, *v)
		checks = append(checks, fmt.Sprintf("(select id is not null from entry_type where id = $%d)", len(args)))
	}
	if v := updata.DateAdded; v != nil {
		e.DateAdded = *v
		set, args = append(set, "date_added = ?"), append(args, *v)
	}
	if v := updata.Quantity; v != nil {
		e.Quantity = v
		set, args = append(set, "quantity = ?"), append(args, *v)
	}
	if v := updata.CompanyID; v != nil {
		e.CompanyID = v
		set, args = append(set, "company_id = ?"), append(args, *v)
		checks = append(checks, fmt.Sprintf("(select id is not null from company where id = $%d)", len(args)))
	}
	if len(args) > 0 {
		replaceQuestionMark(set, args)
	}

	args = append(args, id)
	check := ""
	wherestr := fmt.Sprintf("where entry.id = $%d", len(args))
	if len(checks) > 0 {
		check = " and " + strings.Join(checks, " and ")
	}

	sqlstr := `
		update entry
		set ` + strings.Join(set, ", ") + " " + wherestr + check

	result, err := tx.ExecContext(ctx, sqlstr, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.entry: cannot update %w", err)
	}

	n64, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n64 == 0 {
		return nil, errors.New("company or entry type cannot be part of entry")

	}

	return e, nil
}

func findEntry(ctx context.Context, tx *Tx, filter dots.EntryFilter) (_ []*dots.Entry, n int, err error) {
	where, args := []string{}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}
	if v := filter.EntryTypeID; v != nil {
		where, args = append(where, "entry_type_id = ?"), append(args, *v)
	}
	if v := filter.DateAdded; v != nil {
		where, args = append(where, "date_added = ?"), append(args, *v)
	}
	if v := filter.Quantity; v != nil {
		where, args = append(where, "quantity = ?"), append(args, *v)
	}
	if v := filter.CompanyID; v != nil {
		where, args = append(where, "company_id = ?"), append(args, *v)
	}

	// TODO deal with isDeleted
	/*if filter.IsDeleted {
		if v := filter.DeletedAtFrom; v != nil {
			// >= ? is intentional
			where, args = append(where, "deleted_at >= ?"), append(args, *v)
		}
		if v := filter.DeletedAtTo; v != nil {
			// < ? is intentional
			// avoid double counting exact midnight values
			where, args = append(where, "deleted_at < ?"), append(args, *v)
		}
	}*/

	wherestr := ""
	if len(where) > 0 {
		replaceQuestionMark(where, args)
		wherestr = "where " + strings.Join(where, " and ")
	}

	/*if !filter.IsDeleted {
		where = append(where, "deleted_at is null")
	} else if filter.DeletedAtTo == nil && filter.DeletedAtFrom == nil {
		where = append(where, "deleted_at is not null")
	}*/

	sqlstr := "select id, entry_type_id, date_added, quantity, company_id, count(*) over() from entry " + wherestr + ` ` + formatLimitOffset(filter.Limit, filter.Offset)
	rows, err := tx.QueryContext(
		ctx,
		sqlstr,
		args...,
	)

	if err == sql.ErrNoRows {
		return nil, 0, dots.Errorf(dots.ENOTFOUND, "entry not found")
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	ee := []*dots.Entry{}
	for rows.Next() {
		var e dots.Entry
		err := rows.Scan(&e.ID, &e.EntryTypeID, &e.DateAdded, &e.Quantity, &e.CompanyID, &n)
		if err != nil {
			return nil, 0, err
		}
		ee = append(ee, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return ee, n, nil
}

func deleteEntry(ctx context.Context, tx *Tx, id int, resurect bool) (n int, err error) {
	where := []string{"core.entry.id = $1"}

	kind := "date_trunc('minute', now())::timestamptz"
	if resurect {
		kind = "null"
		where = append(where, "core.entry.deleted_at is not null")
	} else {
		where = append(where, "core.entry.deleted_at is null")
	}

	wherestr := "where " + strings.Join(where, " and ")

	bareEntry := "and not exists(select id from core.drain where drain.entry_id = $1 limit 1)"

	sqlstr := `update core.entry set deleted_at = %s ` + wherestr + " " + bareEntry
	sqlstr = fmt.Sprintf(sqlstr, kind)

	result, err := tx.ExecContext(
		ctx,
		sqlstr,
		id,
	)
	if err != nil {
		return 0, fmt.Errorf("postgres.entry: cannot soft delete %w", err)
	}

	n64, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(n64), nil
}

func deleteEntryPermanently(ctx context.Context, tx *Tx, id int) (n int, err error) {
	where := []string{"core.entry.id = $1"}

	wherestr := "where " + strings.Join(where, " and ")

	bareEntry := "and not exists(select id from core.drain where drain.entry_id = $1 limit 1)"

	sqlstr := `delete from core.entry ` + wherestr + " " + bareEntry

	result, err := tx.ExecContext(
		ctx,
		sqlstr,
		id,
	)
	if err != nil {
		return 0, fmt.Errorf("postgres.entry: cannot soft delete %w", err)
	}

	n64, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(n64), nil
}

func entryBelongsToUser(ctx context.Context, tx *Tx, u ksuid.KSUID, e int) error {
	sqlstr := `select exists(select e.id
from entry e
where e.company_id = any(select id
from company c
where c.tid = $1)
and e.id = $2);
`
	var exists bool
	err := tx.QueryRowContext(ctx, sqlstr, u, e).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return dots.Errorf(dots.EUNAUTHORIZED, "foreign entry")
	}

	return nil
}

func entriesBelongsToCompany(ctx context.Context, tx *Tx, eids []int, cid int) ([]int, error) {
	sqlstr := `select e.id from entry e where e.id = any($1) and e.company_id = $2`

	rows, err := tx.QueryContext(ctx, sqlstr, eids, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ee := []int{}
	for rows.Next() {
		var eid int
		err = rows.Scan(&eid)
		if err != nil {
			return nil, err
		}
		ee = append(ee, eid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ee) == 0 {
		return nil, sql.ErrNoRows
	}

	return ee, nil
}

func entriesBelongsToUser(ctx context.Context, tx *Tx, ee []int) error {
	if len(ee) == 0 {
		return dots.Errorf(dots.EINVALID, "no entries")
	}

	/*
		--- Calculate difference on postgres side but this is not very explicit
		select coalesce(array_agg(wanted), '{}') diff from unnest($2) as wanted where wanted != all(
		select e.id
		from entry e
		where e.company_id = any(select id
		from company c
		where c.tid = $1)
		  and e.id = any($2)
		)*/

	sqlstr := `select json_agg(e.id) as exists
from entry e
where e.company_id = any(select id from company c)
  and e.id = any($1);
`
	var bb []byte
	err := tx.QueryRowContext(ctx, sqlstr, ee).Scan(&bb)
	if err != nil {
		return err
	}

	var exists []int
	if err := json.Unmarshal(bb, &exists); err != nil {
		return err
	}

	if len(exists) == 0 {
		return &dots.Error{
			Code:    dots.EUNAUTHORIZED,
			Message: "foreign entry",
			Data:    map[string]interface{}{"foreign_entries": ee},
		}
	}

	if len(exists) != len(ee) {
		diff := []int{}
		for _, v1 := range ee {
			found := false
			for _, v2 := range exists {
				if v1 == v2 {
					found = true
					break
				}
			}
			if !found {
				diff = append(diff, v1)
			}
		}

		return &dots.Error{
			Code:    dots.EUNAUTHORIZED,
			Message: "foreign entry",
			Data:    map[string]interface{}{"foreign_entries": diff},
		}
	}

	return nil
}
