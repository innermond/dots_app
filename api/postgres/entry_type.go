package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/innermond/dots"
	"github.com/segmentio/ksuid"
)

type EntryTypeService struct {
	db *DB
}

func NewEntryTypeService(db *DB) *EntryTypeService {
	return &EntryTypeService{db: db}
}

func (s *EntryTypeService) CreateEntryType(ctx context.Context, et *dots.EntryType) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		tx.RollbackOrCommit(err)
	}()

	if canerr := dots.CanDoAnything(ctx); canerr == nil {
		return createEntryType(ctx, tx, et)
	}

	if canerr := dots.CanCreateOwn(ctx); canerr != nil {
		return canerr
	}
	et.TID = dots.UserFromContext(ctx).ID

	if err := createEntryType(ctx, tx, et); err != nil {
		return err
	}

	return nil
}

func (s *EntryTypeService) FindEntryType(ctx context.Context, filter dots.EntryTypeFilter) ([]*dots.EntryType, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}

	if canerr := dots.CanDoAnything(ctx); canerr == nil {
		return findEntryType(ctx, tx, filter)
	}

	if canerr := dots.CanReadOwn(ctx); canerr != nil {
		return nil, 0, canerr
	}

	uid := dots.UserFromContext(ctx).ID
	// trying to get entry types for a different TID
	if filter.TID != nil && *filter.TID != uid {
		// will get empty results and not error
		return make([]*dots.EntryType, 0), 0, nil
	}
	// lock search to own
	filter.TID = &uid

	return findEntryType(ctx, tx, filter)
}

func (s *EntryTypeService) UpdateEntryType(ctx context.Context, id int, upd dots.EntryTypeUpdate) (*dots.EntryType, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ee, n, err := s.FindEntryType(ctx, dots.EntryTypeFilter{ID: &id, Limit: 1})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, dots.Errorf(dots.ENOTFOUND, "entry type not found")
	}
	tid := ee[0].TID

	if canerr := dots.CanDoAnything(ctx); canerr == nil {
		return updateEntryType(ctx, tx, id, upd)
	}

	if canerr := dots.CanWriteOwn(ctx, tid); canerr != nil {
		return nil, canerr
	}

	et, err := updateEntryType(ctx, tx, id, upd)
	if err != nil {
		return nil, err
	}

	tx.Commit()

	return et, nil
}

func (s *EntryTypeService) DeleteEntryType(ctx context.Context, filter dots.EntryTypeDelete) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if canerr := dots.CanDoAnything(ctx); canerr == nil {
		return deleteEntryType(ctx, tx, filter)
	}

	if canerr := dots.CanDeleteOwn(ctx); canerr != nil {
		return 0, canerr
	}

	var n int
	// lock delete to own
	uid := dots.UserFromContext(ctx).ID
	filter.TID = &uid

	if filter.ID != nil {
		err = entryTypeBelongsToUser(ctx, tx, uid, *filter.ID)
		if err != nil {
			return 0, err
		}
		n, err = deleteEntryType(ctx, tx, filter)
	} else {
		n, err = deleteEntryType(ctx, tx, filter)
	}

	tx.Commit()

	return n, err
}

func createEntryType(ctx context.Context, tx *Tx, et *dots.EntryType) error {
	user := dots.UserFromContext(ctx)
	if user.ID == ksuid.Nil {
		return dots.Errorf(dots.EUNAUTHORIZED, "unauthorized user")
	}

	if err := et.Validate(); err != nil {
		return err
	}

	sqlstr := `
insert into entry_type
(code, unit, description, tid)
values
($1, $2, $3, $4) returning id
`
	err := tx.QueryRowContext(
		ctx,
		sqlstr,
		et.Code, et.Unit, et.Description, et.TID,
	).Scan(&et.ID)
	if err != nil {
		return err
	}

	return nil
}

func updateEntryType(ctx context.Context, tx *Tx, id int, updata dots.EntryTypeUpdate) (*dots.EntryType, error) {
	ee, _, err := findEntryType(ctx, tx, dots.EntryTypeFilter{ID: &id, Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("postgres.entry type: cannot retrieve entry type %w", err)
	}
	if len(ee) == 0 {
		return nil, dots.Errorf(dots.ENOTFOUND, "entry type not found")
	}
	et := ee[0]

	set, args := []string{}, []interface{}{}
	if v := updata.Code; v != nil {
		et.Code = *v
		set, args = append(set, "code = ?"), append(args, *v)
	}
	if v := updata.Unit; v != nil {
		et.Unit = *v
		set, args = append(set, "unit = ?"), append(args, *v)
	}
	if v := updata.Description; v != nil {
		et.Description = v
		set, args = append(set, "description = ?"), append(args, *v)
	}

	for inx, v := range set {
		v = strings.Replace(v, "?", fmt.Sprintf("$%d", inx+1), 1)
		set[inx] = v
	}
	args = append(args, id)

	sqlstr := `
		update entry_type
		set ` + strings.Join(set, ", ") + `
		where	id = ` + fmt.Sprintf("$%d", len(args))

	_, err = tx.ExecContext(ctx, sqlstr, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.entry type: cannot update %w", err)
	}

	return et, nil
}

func findEntryType(ctx context.Context, tx *Tx, filter dots.EntryTypeFilter) (_ []*dots.EntryType, n int, err error) {
	where, args := []string{"1 = 1"}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}
	if v := filter.Code; v != nil {
		where, args = append(where, "code = ?"), append(args, *v)
	}
	if v := filter.Unit; v != nil {
		where, args = append(where, "unit = ?"), append(args, *v)
	}
	if v := filter.TID; v != nil {
		where, args = append(where, "tid = ?"), append(args, *v)
	}
	for inx, v := range where {
		if !strings.Contains(v, "?") {
			continue
		}
		v = strings.Replace(v, "?", fmt.Sprintf("$%d", inx), 1)
		where[inx] = v
	}

	rows, err := tx.QueryContext(ctx, `
		select id, code, description, unit, tid, count(*) over() from entry_type
		where `+strings.Join(where, " and ")+` `+formatLimitOffset(filter.Limit, filter.Offset),
		args...,
	)
	if err == sql.ErrNoRows {
		return nil, 0, dots.Errorf(dots.ENOTFOUND, "entry type not found")
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entryTypes := []*dots.EntryType{}
	for rows.Next() {
		var et dots.EntryType
		err := rows.Scan(&et.ID, &et.Code, &et.Description, &et.Unit, &et.TID, &n)
		if err != nil {
			return nil, 0, err
		}
		entryTypes = append(entryTypes, &et)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return entryTypes, n, nil
}

func deleteEntryType(ctx context.Context, tx *Tx, filter dots.EntryTypeDelete) (n int, err error) {
	where, args := []string{"1 = 1"}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "et.id = ?"), append(args, *v)
	}
	if v := filter.Code; v != nil {
		where, args = append(where, "code = ?"), append(args, *v)
	}
	if v := filter.Unit; v != nil {
		where, args = append(where, "unit = ?"), append(args, *v)
	}
	if v := filter.TID; v != nil {
		where, args = append(where, "et.tid = ?"), append(args, *v)
	}
	if v := filter.DeletedAtFrom; v != nil {
		// >= ? is intentional
		where, args = append(where, "et.deleted_at >= ?"), append(args, *v)
	}
	if v := filter.DeletedAtTo; v != nil {
		// < ? is intentional
		// avoid double counting exact midnight values
		where, args = append(where, "et.deleted_at < ?"), append(args, *v)
	}
	for inx, v := range where {
		if !strings.Contains(v, "?") {
			continue
		}
		v = strings.Replace(v, "?", fmt.Sprintf("$%d", inx), 1)
		where[inx] = v
	}
	where = append(where, "e.id is null")

	kind := "date_trunc('minute', now())::timestamptz"
	if filter.Resurect {
		kind = "null"
		where = append(where, "et.deleted_at is not null")
	} else {
		where = append(where, "et.deleted_at is null")
	}

	sqlstr := `
		update entry_type set deleted_at = %s where id = any(
		select et.id from entry_type et left join entry e on(et.id = e.entry_type_id)
		where %s)`
	sqlstr = fmt.Sprintf(sqlstr, kind, strings.Join(where, " and ")+` `+formatLimitOffset(filter.Limit, filter.Offset))
	result, err := tx.ExecContext(
		ctx,
		sqlstr,
		args...,
	)
	if err != nil {
		return 0, fmt.Errorf("postgres.entry type: cannot soft delete %w", err)
	}

	n64, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(n64), nil
}

func entryTypeBelongsToUser(ctx context.Context, tx *Tx, u ksuid.KSUID, e int) error {
	sqlstr := `select exists(select e.id
from entry_type e
where e.tid = $1 and e.id = $2);
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