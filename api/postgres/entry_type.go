package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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

func (s *EntryTypeService) CreateEntryType(ctx context.Context, et *dots.EntryType) error {
	if err := et.Validate(); err != nil {
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

	if err := createEntryType(ctx, tx, et); err != nil {
		return perr(err)
	}

	tx.Commit()

	return nil
}

func (s *EntryTypeService) FindEntryType(ctx context.Context, filter dots.EntryTypeFilterOrdered) ([]*dots.EntryType, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}

	if canerr := dots.CanReadOwn(ctx); canerr != nil {
		return nil, 0, canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return nil, 0, err
	}

	return findEntryType(ctx, tx, filter)
}

func (s *EntryTypeService) FindEntryTypeStats(ctx context.Context, filter dots.StatsFilter) (map[string]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	if canerr := dots.CanReadOwn(ctx); canerr != nil {
		return nil, canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return nil, err
	}

	return findEntryTypeStats(ctx, tx, filter)
}

func (s *EntryTypeService) FindEntryTypeUnit(ctx context.Context) ([]string, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}

	if canerr := dots.CanReadOwn(ctx); canerr != nil {
		return nil, 0, canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return nil, 0, err
	}

	return findEntryTypeUnit(ctx, tx)
}

func (s *EntryTypeService) UpdateEntryType(ctx context.Context, id int, upd dots.EntryTypeUpdate) (*dots.EntryType, error) {
	if err := upd.Validate(); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return nil, err
	}

	isDeleted := false
	find := dots.EntryTypeFilterOrdered{
		ID:              []string{strconv.Itoa(id)},
		EntryTypeFilter: dots.EntryTypeFilter{IsDeleted: &isDeleted},
	}
	_, n, err := s.FindEntryType(ctx, find)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, dots.Errorf(dots.ENOTFOUND, "entry type not found")
	}

	if canerr := dots.CanWriteOwn(ctx); canerr != nil {
		return nil, canerr
	}

	et, err := updateEntryType(ctx, tx, id, upd)
	if err != nil {
		return nil, err
	}

	tourist := dots.TouristFromContext(ctx)
	select {
	case <-ctx.Done():
		tx.Rollback()
		fmt.Println("store aborted")
	default:
		tx.Commit()
		tourist <- "storer"
		fmt.Println("store commited")
	}

	return et, nil
}

func (s *EntryTypeService) DeleteEntryType(ctx context.Context, id int, filter dots.EntryTypeDelete) (int, error) {
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
		n, err = deleteEntryTypePermanently(ctx, tx, id)
	} else {
		n, err = deleteEntryType(ctx, tx, id, filter.Resurect)
	}
	if err != nil {
		return n, err
	}

	tx.Commit()

	return n, err

}

func createEntryType(ctx context.Context, tx *Tx, et *dots.EntryType) error {
	sqlstr, args := `
insert into entry_type
(code, unit, description)
values
($1, $2, $3) returning id
`, []interface{}{et.Code, et.Unit, et.Description}

	if err := tx.QueryRowContext(
		ctx,
		sqlstr,
		args...,
	).Scan(&et.ID); err != nil {
		return err
	}

	return nil
}

func updateEntryType(ctx context.Context, tx *Tx, id int, updata dots.EntryTypeUpdate) (*dots.EntryType, error) {
	ee, _, err := findEntryType(ctx, tx, dots.EntryTypeFilterOrdered{EntryTypeFilter: dots.EntryTypeFilter{Limit: 1}, ID: []string{strconv.Itoa(id)}})
	if err != nil {
		return nil, fmt.Errorf("postgres.entry type: cannot retrieve entry type %w", err)
	}
	if len(ee) == 0 {
		return nil, dots.Errorf(dots.ENOTFOUND, "entry type not found")
	}
	et := ee[0]

	set, args := []string{}, []interface{}{}
	if v := updata.Code; v != nil {
		et.Code = v
		set, args = append(set, "code = ?"), append(args, *v)
	}
	if v := updata.Unit; v != nil {
		et.Unit = v
		set, args = append(set, "unit = ?"), append(args, *v)
	}
	if v := updata.Description; v != nil {
		et.Description = v
		set, args = append(set, "description = ?"), append(args, *v)
	}
	replaceQuestionMark(set, args)
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

func applyMask(fieldname string, mask string, v []string) (where []string, args []interface{}, order []string) {
	value, kind := "", ""
	for i, m := range mask {
		switch m {
		case 'v':
			value = v[i]
		case 'o':
			o := "desc"
			if v[i] == "1" {
				o = "asc"
			} else if v[i] == "-1" {
				o = "desc"
			}
			order = append(order, fmt.Sprintf("%s %s", fieldname, o))
		case 'k':
			kind = v[i]
		}
	}
	if value != "" {
		switch kind {
		case "0": // start
			where, args = append(where, fieldname+" like ? || '%'"), append(args, value)
		case "2": // end
			where, args = append(where, fieldname+" like '%' || ?"), append(args, value)
		case "1": // middle
			where, args = append(where, fieldname+" like '%' || ? || '%'"), append(args, value)
			where, args = append(where, fieldname+" not like ? || '%'"), append(args, value)
		default:
			where, args = append(where, fieldname+" = ?"), append(args, value)
		}
	}
	if kind == "3" { // empty
		where = append(where, fmt.Sprintf(" %s is null or %s = ''", fieldname, fieldname))
	}

	return
}

func findEntryType(ctx context.Context, tx *Tx, filter dots.EntryTypeFilterOrdered) (_ []*dots.EntryType, n int, err error) {
	where, args, order := []string{}, []interface{}{}, []string{}
	if v := filter.ID; len(v) > 0 {
		if filter.MaskID != "" {
			w, a, o := applyMask("id", filter.MaskID, v)
			where = append(where, w...)
			args = append(args, a...)
			order = append(order, o...)
		} else {
			where, args = append(where, "id = ?"), append(args, v[0])
		}
	}
	if v := filter.Code; len(v) > 0 {
		if filter.MaskCode != "" {
			w, a, o := applyMask("code", filter.MaskCode, v)
			where = append(where, w...)
			args = append(args, a...)
			order = append(order, o...)
		} else {
			where, args = append(where, "code = ?"), append(args, v[0])
		}
	}
	if v := filter.Description; len(v) > 0 {
		if filter.MaskDescription != "" {
			w, a, o := applyMask("description", filter.MaskDescription, v)
			where = append(where, w...)
			args = append(args, a...)
			order = append(order, o...)
		} else {
			where, args = append(where, "description = ?"), append(args, v[0])
		}
	}

	if v := filter.Unit; len(v) > 0 {
		if filter.MaskUnit != "" {
			w, a, o := applyMask("unit", filter.MaskUnit, v)
			where = append(where, w...)
			args = append(args, a...)
			order = append(order, o...)
		} else {
			where, args = append(where, "unit = ?"), append(args, v[0])
		}
	}

	wherestr := ""
	if len(where) > 0 {
		replaceQuestionMark(where, args)
		wherestr = "where " + strings.Join(where, " and ")
	}
	limitoffset := formatLimitOffset(filter.Limit, filter.Offset)
	orderstr := ""
	if len(order) > 0 {
		orderstr = "order by " + strings.Join(order, ", ")
	}
	sqlstr := `select id, code, description, unit, count(*) over() from entry_type
	` + wherestr + " " + orderstr + " " + limitoffset

	fmt.Println(sqlstr, args)

	rows, err := tx.QueryContext(ctx,
		sqlstr,
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
	empty := ""
	for rows.Next() {
		var et dots.EntryType
		err := rows.Scan(&et.ID, &et.Code, &et.Description, &et.Unit, &n)
		if err != nil {
			return nil, 0, err
		}
		// TODO implementing default value "" at database level?
		if et.Description == nil {
			et.Description = &empty
		}
		entryTypes = append(entryTypes, &et)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return entryTypes, n, nil
}

func findEntryTypeStats(ctx context.Context, tx *Tx, filter dots.StatsFilter) (out map[string]string, err error) {
	kind := "default"
	if v := filter.Kind; v != nil {
		kind = *v
	}

	where, args := []string{}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "et.id = ?"), append(args, *v)
	}

	wherestr := ""
	if len(where) > 0 {
		replaceQuestionMark(where, args)
		wherestr = "where " + strings.Join(where, " and ")
	}
	// default stats
	if kind != "default" {
		panic("not implemented")
	}

	sqlstr := `select
	count(e.id) as entry_count,
	c.longname as company_name
from
	api.entry e
join
    api.entry_type et on
	e.entry_type_id = et.id
join
    api.company c on
	c.id = e.company_id ` + wherestr + `
group by
    c.id, c.longname;`
	rows, err := tx.QueryContext(ctx,
		sqlstr,
		args...,
	)
	if err == sql.ErrNoRows {
		return nil, dots.Errorf(dots.ENOTFOUND, "entry type stats not found")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]string)
	num := ""
	name := ""
	for rows.Next() {
		err := rows.Scan(&num, &name)
		if err != nil {
			return nil, err
		}
		stats[name] = fmt.Sprint(num)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func findEntryTypeUnit(ctx context.Context, tx *Tx) (_ []string, n int, err error) {
	sqlstr := "select entry_type.unit from entry_type group by unit"
	rows, err := tx.QueryContext(ctx, sqlstr)
	if err == sql.ErrNoRows {
		return nil, 0, dots.Errorf(dots.ENOTFOUND, "entry type unit not found")
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entryTypeUnits := []string{}
	for rows.Next() {
		var u string
		err := rows.Scan(&u)
		if err != nil {
			return nil, 0, err
		}
		entryTypeUnits = append(entryTypeUnits, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	n = len(entryTypeUnits)

	return entryTypeUnits, n, nil
}

func deleteEntryType(ctx context.Context, tx *Tx, id int, resurect bool) (n int, err error) {
	where := []string{"core.entry_type.id = $1"}

	kind := "date_trunc('minute', now())::timestamptz"
	if resurect {
		kind = "null"
		where = append(where, "core.entry_type.deleted_at is not null")
	} else {
		where = append(where, "core.entry_type.deleted_at is null")
	}

	wherestr := "where " + strings.Join(where, " and ")

	bareEntryType := `
		and not exists(
		select et.id from entry_type et join entry e on(et.id = e.entry_type_id)
		where e.entry_type_id = $1 limit 1)
`
	sqlstr := `update core.entry_type set deleted_at = %s  ` + wherestr + bareEntryType
	sqlstr = fmt.Sprintf(sqlstr, kind)

	result, err := tx.ExecContext(
		ctx,
		sqlstr,
		id,
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

func deleteEntryTypePermanently(ctx context.Context, tx *Tx, id int) (n int, err error) {
	where := []string{"core.entry_type.id = $1"}
	wherestr := "where " + strings.Join(where, " and ")

	bareEntryType := `
		and not exists(
		select et.id from entry_type et join entry e on(et.id = e.entry_type_id)
		where e.entry_type_id = $1 limit 1)
`
	sqlstr := `delete from core.entry_type ` + wherestr + bareEntryType

	result, err := tx.ExecContext(
		ctx,
		sqlstr,
		id,
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
