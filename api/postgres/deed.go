package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/innermond/dots"
	"github.com/segmentio/ksuid"
)

var (
	ErrNotFound = dots.Errorf(dots.ENOTFOUND, "deed not found")
)

type DeedService struct {
	db *DB
}

func NewDeedService(db *DB) *DeedService {
	return &DeedService{db: db}
}

func (s *DeedService) CreateDeed(ctx context.Context, d *dots.Deed) error {
	if err := d.Validate(); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if d.CompanyID == nil {
		return dots.Errorf(dots.ENOTFOUND, "company is required")
	}

	if canerr := dots.CanCreateOwn(ctx); canerr != nil {
		return canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return err
	}

	filterFind := dots.CompanyFilter{ID: d.CompanyID}
	_, n, err := findCompany(ctx, tx, filterFind)
	if err != nil {
		return err
	}
	if n == 0 {
		return dots.Errorf(dots.ENOTFOUND, "company not found %v", *d.CompanyID)
	}

	if err := doDistribute(ctx, tx, &d.DeedUpdate); err != nil {
		return err
	}

	if err := createDeed(ctx, tx, d); err != nil {
		return err
	}

	tx.Commit()

	return nil
}

func (s *DeedService) FindDeed(ctx context.Context, filter dots.DeedFilter) ([]*dots.Deed, int, error) {
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

	if filter.CompanyID != nil {
		err := companyBelongsToUser(ctx, tx, *filter.CompanyID)
		if err != nil {
			return nil, 0, err
		}
	}

	return findDeed(ctx, tx, filter)
}

func (s *DeedService) UpdateDeed(ctx context.Context, id int, upd dots.DeedUpdate) (*dots.Deed, error) {
	// validation
	if upd.CompanyID == nil {
		return nil, dots.Errorf(dots.ENOTFOUND, "company is required")
	}

	if len(upd.Distribute) > 0 {
		for eid, qty := range upd.Distribute {
			if qty <= 0 {
				return nil, dots.Errorf(dots.EINVALID, "quantity for entry %d must be greater than zero", eid)
			}
		}
	}

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

	filterFind := dots.CompanyFilter{ID: upd.CompanyID}
	_, n, err := findCompany(ctx, tx, filterFind)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, dots.Errorf(dots.ENOTFOUND, "company not found %v", *upd.CompanyID)
	}

	d, err := updateDeed(ctx, tx, id, upd)
	if err != nil {
		return nil, err
	}

	tx.Commit()

	return d, nil
}

func (s *DeedService) DeleteDeed(ctx context.Context, id int, filter dots.DeedDelete) (int, error) {
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

	n, err = deleteDeed(ctx, tx, id, filter)

	tx.Commit()

	return n, err
}

func createDeed(ctx context.Context, tx *Tx, d *dots.Deed) error {
	err := tx.QueryRowContext(
		ctx,
		`
insert into deed
(title, quantity, unit, unitprice, company_id)
values
($1, $2, $3, $4, $5) returning id
		`,
		d.Title, d.Quantity, d.Unit, d.UnitPrice, d.CompanyID,
	).Scan(&d.ID)
	if err != nil {
		return err
	}

	if len(d.Distribute) == 0 && len(d.EntryTypeDistribute) == 0 {
		return nil
	}

	if err := doDistribute(ctx, tx, &d.DeedUpdate); err != nil {
		return err
	}

	// manage distribute
	for eid, qty := range d.Distribute {
		d := dots.Drain{
			DeedID:    *d.ID,
			EntryID:   eid,
			Quantity:  qty,
			IsDeleted: false,
		}

		err = createOrUpdateDrain(ctx, tx, d)
		if err != nil {
			// all or nothing
			return err
		}

	}

	return nil
}

func updateDeed(ctx context.Context, tx *Tx, id int, upd dots.DeedUpdate) (*dots.Deed, error) {
	dd, _, err := findDeed(ctx, tx, dots.DeedFilter{ID: &id, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(dd) == 0 {
		return nil, ErrNotFound
	}
	e := dd[0]
	oldCompanyID := *e.CompanyID

	set, args := []string{}, []interface{}{}
	if v := upd.Title; v != nil {
		e.Title = v
		set, args = append(set, "title = ?"), append(args, *v)
	}
	if v := upd.Quantity; v != nil {
		e.Quantity = v
		set, args = append(set, "quantity = ?"), append(args, *v)
	}
	if v := upd.Unit; v != nil {
		e.Unit = v
		set, args = append(set, "unit = ?"), append(args, *v)
	}
	if v := upd.UnitPrice; v != nil {
		e.UnitPrice = v
		set, args = append(set, "unitprice = ?"), append(args, *v)
	}
	if v := upd.CompanyID; v != nil {
		// assumed that drains's issue has been addresed by caller
		e.CompanyID = v
		set, args = append(set, "company_id = ?"), append(args, *v)
	}

	replaceQuestionMark(set, args)
	args = append(args, id)

	sqlstr := `
		update deed
		set ` + strings.Join(set, ", ") + `
		where	id = ` + fmt.Sprintf("$%d", len(args))

	result, err := tx.ExecContext(ctx, sqlstr, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.deed: cannot update %w", err)
	}
	n64, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n64 == 0 {
		return nil, dots.Errorf(dots.ENOTAFFECTED, "deed %d not affected", *e.ID)
	}

	// we will do some distribution
	if len(upd.Distribute) > 0 || len(upd.EntryTypeDistribute) > 0 {
		// soft delete all drains of this deed
		// to allow corect calculation of distribution
		// as update assumes we invalidate previous distribution
		// all quantities drained are "returned"
		if oldCompanyID == *e.CompanyID {
			// hard delete the "deleted" drains
			err = hardDeleteDrainsOfDeedAlreadyDeleted(ctx, tx, *e.ID)
			if err != nil {
				return nil, err
			}
			// and "delete" all active drains
			err = deleteDrainsOfDeed(ctx, tx, id)
			if err != nil {
				return nil, err
			}
		} else {
			// remove all drains of deed
			err = hardDeleteDrainsOfDeed(ctx, tx, *e.ID)
			if err != nil {
				return nil, err
			}
		}

		if err := doDistribute(ctx, tx, &upd); err != nil {
			return nil, err
		}
	}

	if len(upd.Distribute) == 0 {
		// find undeleted drains
		filter := dots.DrainFilter{DeedID: &id}
		drains, n, err := findDrain(ctx, tx, filter)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			distribute := map[int]float64{}
			for _, drain := range drains {
				distribute[drain.EntryID] = drain.Quantity
			}
			e.Distribute = distribute
		}
		return e, nil
	}

	for eid, qty := range upd.Distribute {
		d := dots.Drain{
			DeedID:    *e.ID,
			EntryID:   eid,
			Quantity:  qty,
			IsDeleted: false,
		}

		err = createOrUpdateDrain(ctx, tx, d)
		if err != nil {
			// all or nothing
			return nil, err
		}
	}

	e.Distribute = upd.Distribute

	return e, nil
}

func findDeed(ctx context.Context, tx *Tx, filter dots.DeedFilter) (_ []*dots.Deed, n int, err error) {
	where, args := []string{}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}
	if v := filter.Title; v != nil {
		where, args = append(where, "title = ?"), append(args, *v)
	}
	if v := filter.Quantity; v != nil {
		where, args = append(where, "quantity = ?"), append(args, *v)
	}
	if v := filter.Unit; v != nil {
		where, args = append(where, "unit = ?"), append(args, *v)
	}
	if v := filter.UnitPrice; v != nil {
		where, args = append(where, "unitprice = ?"), append(args, *v)
	}
	/*	if v := filter.DeletedAtFrom; v != nil {
			// >= ? is intentional
			where, args = append(where, "deleted_at >= ?"), append(args, *v)
		}
		if v := filter.DeletedAtTo; v != nil {
			// < ? is intentional
			// avoid double counting exact midnight values
			where, args = append(where, "deleted_at < ?"), append(args, *v)
		}
	*/
	if v := filter.CompanyID; v != nil {
		where, args = append(where, "company_id = ?"), append(args, *v)
	}
	replaceQuestionMark(where, args)

	// WARN: placeholder ? is connected with position in "where"
	// so any unrelated with position (read replacement $n)
	// MUST be added AFTER the "for" cycle
	// that binds value with placeholder
	if filter.CompanyID == nil {
		where = append(where, "company_id = any(select id from company)")
	}

	sqlstr := `select id, title, unit, unitprice, quantity, company_id, count(*) over() from deed
		where `
	sqlstr = sqlstr + strings.Join(where, " and ") + ` ` + formatLimitOffset(filter.Limit, filter.Offset)
	rows, err := tx.QueryContext(
		ctx,
		sqlstr,
		args...,
	)
	if err == sql.ErrNoRows {
		return nil, 0, dots.Errorf(dots.ENOTFOUND, "deed not found")
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	deeds := []*dots.Deed{}
	for rows.Next() {
		var d dots.Deed
		err := rows.Scan(&d.ID, &d.Title, &d.Unit, &d.UnitPrice, &d.Quantity, &d.CompanyID, &n)
		if err != nil {
			return nil, 0, err
		}
		deeds = append(deeds, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return deeds, n, nil
}

func deleteDeed(ctx context.Context, tx *Tx, id int, filter dots.DeedDelete) (n int, err error) {
	where, args := []string{}, []interface{}{}
	where, args = append(where, "id = ?"), append(args, id)

	replaceQuestionMark(where, args)

	kind := "date_trunc('minute', now())::timestamptz"
	if filter.Resurect {
		kind = "null"
		where = append(where, "deleted_at is not null")
	} else {
		where = append(where, "deleted_at is null")
	}
	sqlstr := "update core.deed set deleted_at = " + kind + " where "
	sqlstr = sqlstr + strings.Join(where, " and ")

	result, err := tx.ExecContext(
		ctx,
		sqlstr,
		args...,
	)
	if err != nil {
		return 0, fmt.Errorf("postgres.deed: cannot soft delete %w", err)
	}

	if filter.Undrain {
		err := changeDrainsOfDeed(ctx, tx, id, !filter.Resurect)
		if err != nil {
			return 0, fmt.Errorf("postgres.deed: cannot undrain %w", err)
		}
	}

	n64, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(n64), nil
}

func deedBelongsToUser(ctx context.Context, tx *Tx, u ksuid.KSUID, d int) error {
	sqlstr := `select exists(select d.id
from deed d
where d.company_id = any(select id
from company c
where c.tid = $1)
and d.id = $2);
`
	var exists bool
	err := tx.QueryRowContext(ctx, sqlstr, u, d).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return dots.Errorf(dots.EUNAUTHORIZED, "foreign deed")
	}

	return nil
}

func entriesOfCompanyAreEnough(ctx context.Context, tx *Tx, eq map[int]float64, cid int) (map[int]float64, error) {
	eids := keysOf(eq)

	belong, err := entriesBelongsToCompany(ctx, tx, eids, cid)
	if err != nil {
		return nil, err
	}
	if len(belong) != len(eids) {
		notbelong := []int{}
		for _, eid := range eids {
			found := false
			for _, beid := range belong {
				if eid != beid {
					continue
				}
				found = true
				break
			}
			if !found {
				notbelong = append(notbelong, eid)
			}
		}
		errd := dots.Errorf(dots.ENOTFOUND, "some entries do not belong")
		errd.Data = map[string]interface{}{"entry_id": notbelong, "company_id": cid}
		return nil, errd
	}

	eidqty, err := quantityByEntries(ctx, tx, eids, cid)
	if err != nil {
		if err == sql.ErrNoRows {
			errd := dots.Errorf(dots.ENOTFOUND, "entries not found")
			errd.Data = map[string]interface{}{"entry_id": eids, "company_id": cid}
			return nil, errd
		}
		return nil, err
	}

	needmore := map[int]float64{}
	for k, wanted := range eq {
		if existent, found := eidqty[k]; !found {
			return nil, dots.Errorf(dots.ENOTFOUND, "not found entry %v", k)
		} else if wanted > existent {
			needmore[k] = wanted - existent
		}
	}

	if len(needmore) > 0 {
		err := dots.Errorf(dots.EINVALID, "not enough quantity")
		err.Data = map[string]interface{}{"needmore": needmore}
		return nil, err
	}

	return eidqty, nil
}

func keysOf[K, V comparable](ee map[K]V) []K {
	if len(ee) == 0 {
		return []K{}
	}

	ids := []K{}
	for id := range ee {
		ids = append(ids, id)
	}

	return ids
}

type entryRow struct {
	eid  int
	etid int
	qty  float64
}

func doDistribute(ctx context.Context, tx *Tx, upd *dots.DeedUpdate) error {
	// try first automatic distribute
	enoughChecked := false
	if len(upd.EntryTypeDistribute) > 0 {
		strategy := ""
		if upd.DistributeStrategy != nil {
			strategy = string(*upd.DistributeStrategy)
		}

		distribute, err := tryDistributeOverEntryType(ctx, tx, upd.EntryTypeDistribute, *upd.CompanyID, strategy)
		if err != nil {
			return err
		}
		upd.Distribute = distribute
		enoughChecked = true
	}

	// accept only to distribute by entry IDs
	if len(upd.Distribute) > 0 {
		var err error
		check := map[int]float64{}
		if !enoughChecked {
			// check entries are owned and enough
			check, err = entriesOfCompanyAreEnough(ctx, tx, upd.Distribute, *upd.CompanyID)
			if err != nil {
				if err == sql.ErrNoRows {
					return dots.Errorf(dots.ENOTFOUND, "entries owned and enough not found")
				}
				return err
			}
		}
		// need to check check
		needmore := map[int]float64{}
		for eid, diff := range check {
			if diff < 0 {
				needmore[eid] = diff
			}
		}
		// not enough
		if len(needmore) > 0 {
			err := &dots.Error{
				Code:    dots.ECONFLICT,
				Message: "not enough entries",
				Data:    map[string]interface{}{"needmore": needmore, "company_id": *upd.CompanyID},
			}
			return err
		}
	}
	return nil
}
