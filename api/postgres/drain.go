package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/innermond/dots"
)

type DrainService struct {
	db *DB
}

func NewDrainService(db *DB) *DrainService {
	return &DrainService{db: db}
}

func (s *DrainService) CreateOrUpdateDrain(ctx context.Context, d dots.Drain) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if canerr := dots.CanDoAnything(ctx); canerr == nil {
		return createOrUpdateDrain(ctx, tx, d)
	}

	if canerr := dots.CanCreateOwn(ctx); canerr != nil {
		return canerr
	}

	// lock create to own
	// need deed ID and entry ID that belong to companies of user
	uid := dots.UserFromContext(ctx).ID
	err = entryBelongsToUser(ctx, tx, uid, d.EntryID)
	if err != nil {
		return err
	}
	err = deedBelongsToUser(ctx, tx, uid, d.DeedID)
	if err != nil {
		return err
	}

	if err := createOrUpdateDrain(ctx, tx, d); err != nil {
		return err
	}

	tx.Commit()

	return nil
}

func (s *DrainService) FindDrain(ctx context.Context, filter dots.DrainFilter) ([]*dots.Drain, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	if canerr := dots.CanDoAnything(ctx); canerr == nil {
		return findDrain(ctx, tx, filter)
	}

	if canerr := dots.CanReadOwn(ctx); canerr != nil {
		return nil, 0, canerr
	}

	uid := dots.UserFromContext(ctx).ID
	// trying to get companies for a different TID
	if filter.TID != nil && *filter.TID != uid {
		// will get empty results and not error
		return make([]*dots.Drain, 0), 0, nil
	}
	// lock search to own
	filter.TID = &uid

	return findDrain(ctx, tx, filter)
}

func createOrUpdateDrain(ctx context.Context, tx *Tx, d dots.Drain) error {
	if err := d.Validate(); err != nil {
		return err
	}

	sqlstr := `
insert into core.drain
(deed_id, entry_id, quantity, is_deleted)
values
($1, $2, $3, $4)
on conflict (deed_id, entry_id) do update set deed_id = EXCLUDED.deed_id, entry_id = EXCLUDED.entry_id, quantity = EXCLUDED.quantity, is_deleted = EXCLUDED.is_deleted
		`
	_, err := tx.ExecContext(
		ctx,
		sqlstr,
		d.DeedID, d.EntryID, d.Quantity, d.IsDeleted,
	)

	if err != nil {
		return err
	}

	return nil
}

func deleteDrainsOfDeed(ctx context.Context, tx *Tx, id int) error {
	return changeDrainsOfDeed(ctx, tx, id, true)
}

func changeDrainsOfDeed(ctx context.Context, tx *Tx, id int, del bool) error {
	_, err := tx.ExecContext(
		ctx,
		"update core.drain set is_deleted = $2 where deed_id = $1",
		id, del,
	)
	if err != nil {
		return err
	}

	return nil
}

func undrainDrainsOfDeed(ctx context.Context, tx *Tx, id int) error {
	_, err := tx.ExecContext(
		ctx,
		"update core.drain set is_deleted = not is_deleted where deed_id = $1",
		id,
	)
	if err != nil {
		return err
	}

	return nil

}

func hardDeleteDrainsOfDeed(ctx context.Context, tx *Tx, did int) error {
	sqlstr := `delete from core.drain where deed_id = $1`

	_, err := tx.ExecContext(ctx, sqlstr, did)
	if err != nil {
		return err
	}

	return nil
}

func hardDeleteDrainsOfDeedAlreadyDeleted(ctx context.Context, tx *Tx, did int) error {
	sqlstr := `delete from core.drain d where d.deed_id = $1 and d.is_deleted = true`

	_, err := tx.ExecContext(ctx, sqlstr, did)
	if err != nil {
		return err
	}

	return nil
}

func hardDeleteDrainsOfDeedPrevCompany(ctx context.Context, tx *Tx, did, cid int) error {
	sqlstr := `delete from core.drain d where d.deed_id = $1 and d.entry_id = any(select e.id from entry e where e.company_id = $2)`

	_, err := tx.ExecContext(ctx, sqlstr, did, cid)
	if err != nil {
		return err
	}

	return nil
}

func findDrain(ctx context.Context, tx *Tx, filter dots.DrainFilter) (_ []*dots.Drain, n int, err error) {
	where, args := []string{}, []interface{}{}
	if v := filter.DeedID; v != nil {
		where, args = append(where, "deed_id = ?"), append(args, *v)
	}
	if v := filter.EntryID; v != nil {
		where, args = append(where, "entry_id = ?"), append(args, *v)
	}

	replaceQuestionMark(where, args)

	v := filter.IsDeleted
	if v != nil {
		where = append(where, "is_deleted = "+strconv.FormatBool(*filter.IsDeleted))
	} else {
		where = append(where, "is_deleted = false")
	}

	sqlstr := `
		select d.deed_id, d.entry_id, d.quantity, d.is_deleted, count(*) over() from core.drain d
		where ` + strings.Join(where, " and ") + ` ` + formatLimitOffset(filter.Limit, filter.Offset)

	rows, err := tx.QueryContext(
		ctx,
		sqlstr,
		args...,
	)

	if err == sql.ErrNoRows {
		return nil, 0, dots.Errorf(dots.ENOTFOUND, "drain not found")
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	drains := []*dots.Drain{}
	for rows.Next() {
		var e dots.Drain
		err := rows.Scan(&e.DeedID, &e.EntryID, &e.Quantity, &e.IsDeleted, &n)
		if err != nil {
			return nil, 0, err
		}
		drains = append(drains, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return drains, n, nil
}
