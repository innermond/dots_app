package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/innermond/dots"
)

type CompanyService struct {
	db *DB
}

func NewCompanyService(db *DB) *CompanyService {
	return &CompanyService{db: db}
}

func (s *CompanyService) CreateCompany(ctx context.Context, c *dots.Company) error {
	if err := c.Validate(); err != nil {
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

	if err := createCompany(ctx, tx, c); err != nil {
		return perr(err)
	}

	tx.Commit()

	return nil
}

func (s *CompanyService) FindCompany(ctx context.Context, filter dots.CompanyFilter) ([]*dots.Company, int, error) {
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

	return findCompany(ctx, tx, filter)
}

func (s *CompanyService) UpdateCompany(ctx context.Context, id int, upd dots.CompanyUpdate) (*dots.Company, error) {
	if err := upd.Validate(); err != nil {
		return nil, err
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

	c, err := updateCompany(ctx, tx, id, upd)
	if err != nil {
		return nil, err
	}

	tx.Commit()

	return c, nil
}

func (s *CompanyService) DeleteCompany(ctx context.Context, id int, filter dots.CompanyDelete) (int, error) {
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

	n := 0
	tpl := "company %d not affected"
	if filter.Hard {
		n, err = deleteCompanyPermanently(ctx, tx, id)
		tpl += " (hard)"
	} else {
		n, err = deleteCompany(ctx, tx, id, filter.Resurect)
		if filter.Resurect {
			tpl += " (resurect)"
		} else {
			tpl += " (soft)"
		}
	}
	if err != nil {
		return n, err
	}
	if n == 0 {
		return 0, dots.Errorf(dots.ENOTAFFECTED, tpl, id)
	}

	tx.Commit()

	return n, err
}

func (s *CompanyService) StatsCompany(ctx context.Context, filter dots.CompanyFilter) (*dots.CompanyStats, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if canerr := dots.CanReadOwn(ctx); canerr != nil {
		return nil, canerr
	}

	if err := tx.setUserIDPerConnection(ctx); err != nil {
		return nil, err
	}

	return statsCompany(ctx, tx, filter)
}

func (s *CompanyService) DepletionCompany(ctx context.Context, filter dots.CompanyFilter) ([]*dots.CompanyDepletion, int, error) {
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

	return depletionCompany(ctx, tx, filter)
}

func findCompany(ctx context.Context, tx *Tx, filter dots.CompanyFilter) (_ []*dots.Company, n int, err error) {
	where, args := []string{}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}
	if v := filter.Longname; v != nil {
		where, args = append(where, "longname = ?"), append(args, *v)
	}
	if v := filter.TIN; v != nil {
		where, args = append(where, "tin = ?"), append(args, *v)
	}
	if v := filter.RN; v != nil {
		where, args = append(where, "rn = ?"), append(args, *v)
	}

	// TODO deal with isDeleted
	/*	v := filter.IsDeleted
		if v != nil && *v {
			where = append(where, "deleted_at is not null")
		} else if v != nil && !*v {
			where = append(where, "deleted_at is null")
		} else if v == nil {
			where = append(where, "deleted_at is null")
		}
	*/
	wherestr := ""
	if len(where) > 0 {
		replaceQuestionMark(where, args)
		wherestr = "where " + strings.Join(where, " and ")
	}
	sqlstr := `
		select id, longname, tin, rn, count(*) over() from company
		` + wherestr + ` ` + formatLimitOffset(filter.Limit, filter.Offset)
	rows, err := tx.QueryContext(
		ctx,
		sqlstr,
		args...,
	)
	if err == sql.ErrNoRows {
		return nil, 0, dots.Errorf(dots.ENOTFOUND, "company not found")
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	companies := []*dots.Company{}
	for rows.Next() {
		var e dots.Company
		err := rows.Scan(&e.ID, &e.Longname, &e.TIN, &e.RN, &n)
		if err != nil {
			return nil, 0, err
		}
		companies = append(companies, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return companies, n, nil
}

func createCompany(ctx context.Context, tx *Tx, c *dots.Company) error {
	sqlstr, args := `
insert into company
(longname, tin, rn)
values
($1, $2, $3) returning id
	`, []interface{}{c.Longname, c.TIN, c.RN}

	if err := tx.QueryRowContext(
		ctx,
		sqlstr,
		args...,
	).Scan(&c.ID); err != nil {
		return err
	}

	return nil
}

func updateCompany(ctx context.Context, tx *Tx, id int, updata dots.CompanyUpdate) (*dots.Company, error) {
	cc, _, err := findCompany(ctx, tx, dots.CompanyFilter{ID: &id, Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("postgres.company: cannot retrieve company type %w", err)
	}
	if len(cc) == 0 {
		return nil, dots.Errorf(dots.ENOTFOUND, "company not found")
	}
	ct := cc[0]

	set, args := []string{}, []interface{}{}

	if v := updata.Longname; v != nil {
		ct.Longname = *v
		set, args = append(set, "longname = ?"), append(args, *v)
	}
	if v := updata.TIN; v != nil {
		ct.TIN = *v
		set, args = append(set, "tin = ?"), append(args, *v)
	}
	if v := updata.RN; v != nil {
		ct.RN = *v
		set, args = append(set, "rn = ?"), append(args, *v)
	}

	for inx, v := range set {
		v = strings.Replace(v, "?", fmt.Sprintf("$%d", inx+1), 1)
		set[inx] = v
	}
	args = append(args, id)

	sqlstr := `
		update company
		set ` + strings.Join(set, ", ") + `
		where	id = ` + fmt.Sprintf("$%d", len(args))

	_, err = tx.ExecContext(ctx, sqlstr, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.company: cannot update %w", err)
	}

	return ct, nil
}

func deleteCompany(ctx context.Context, tx *Tx, id int, resurect bool) (n int, err error) {
	where := []string{"core.company.id = $1"}

	kind := "date_trunc('minute', now())::timestamptz"
	if resurect {
		kind = "null"
		where = append(where, "core.company.deleted_at is not null")
	} else {
		where = append(where, "core.company.deleted_at is null")
	}

	wherestr := "where " + strings.Join(where, " and ")

	bareCompany := `
	and not exists(
		select c.id from core.company c join core.entry e on(c.id = e.company_id)
		where e.company_id = $1 limit 1) 
	and not exists(
		select c.id from core.company c join core.deed d on(c.id = d.company_id)
		where d.company_id = $1 limit 1)`

	sqlstr := `update core.company set deleted_at = %s ` + wherestr + bareCompany
	sqlstr = fmt.Sprintf(sqlstr, kind)

	result, err := tx.ExecContext(
		ctx,
		sqlstr,
		id,
	)
	if err != nil {
		return 0, fmt.Errorf("postgres.company: cannot soft delete %w", err)
	}

	n64, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(n64), nil
}

func deleteCompanyPermanently(ctx context.Context, tx *Tx, id int) (n int, err error) {
	where := []string{"core.company.id = $1"}
	wherestr := "where " + strings.Join(where, " and ")

	bareCompany := `
	and not exists(
		select c.id from core.company c join core.entry e on(c.id = e.company_id)
		where e.company_id = $1 limit 1) 
	and not exists(
		select c.id from core.company c join core.deed d on(c.id = d.company_id)
		where d.company_id = $1 limit 1)`

	sqlstr := `delete from core.company ` + wherestr + bareCompany

	result, err := tx.ExecContext(
		ctx,
		sqlstr,
		id,
	)
	if err != nil {
		return 0, fmt.Errorf("postgres.company: cannot hard delete %w", err)
	}

	n64, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(n64), nil
}

func companyBelongsToUser(ctx context.Context, tx *Tx, companyID int) error {
	sqlstr := `select exists(
select id
from company c
where c.id = $1
);
`
	var exists bool
	err := tx.QueryRowContext(ctx, sqlstr, companyID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return dots.Errorf(dots.ENOTFOUND, "company not found")
	}

	return nil
}

func statsCompany(ctx context.Context, tx *Tx, filter dots.CompanyFilter) (_ *dots.CompanyStats, err error) {
	where, args := []string{}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}
	if v := filter.Longname; v != nil {
		where, args = append(where, "longname = ?"), append(args, *v)
	}
	if v := filter.TIN; v != nil {
		where, args = append(where, "tin = ?"), append(args, *v)
	}
	if v := filter.RN; v != nil {
		where, args = append(where, "rn = ?"), append(args, *v)
	}

	wherestr := ""
	if len(where) > 0 {
		replaceQuestionMark(where, args)
		wherestr = "where " + strings.Join(where, " and ")
	}
	sqlstr := `WITH c AS
(SELECT id FROM api.company ` + wherestr + `)
SELECT
  count_companies,
  count_deeds,
  count_entries,
  count_entry_type
FROM (
  SELECT
    (SELECT count(*) FROM c) AS count_companies,
    (SELECT count(*) FROM api.deed WHERE company_id = any(SELECT id FROM c)) AS count_deeds,
    (SELECT count(*) FROM api.entry WHERE company_id IN (SELECT id FROM c)) AS count_entries,
    (SELECT count(*) FROM api.entry_type) AS count_entry_type
) counts;`

	stats := &dots.CompanyStats{}
	err = tx.QueryRowContext(
		ctx,
		sqlstr,
		args...,
	).Scan(&stats.CountCompanies, &stats.CountDeeds, &stats.CountEntries, &stats.CountEntryTypes)
	if err == sql.ErrNoRows {
		return nil, dots.Errorf(dots.ENOTFOUND, "company not found")
	}
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func depletionCompany(ctx context.Context, tx *Tx, filter dots.CompanyFilter) (_ []*dots.CompanyDepletion, n int, err error) {
	cc, n, err := findCompany(ctx, tx, filter)
	if err != nil {
		return nil, 0, err
	}

	cids := []int{}
	for _, c := range cc {
		cids = append(cids, c.ID)
	}

	wherestr := "where ed.company_id = any($1)"

	sqlstr := `with er as (
	select
		ed.id,
		ed.entry_type_id,
		et.code,
		et.description,
		ed.quantity_initial,
		ed.quantity_drained,
		(ed.quantity_initial - ed.quantity_drained) as remained
	from
		api.entry_with_quantity_drained ed
	join api.entry_type et on
		ed.entry_type_id = et.id
		` + wherestr + `
  order by remained
)
select
	er.entry_type_id,
	er.code, er.description,
	Sum(quantity_initial) quantity_initial,
	sum(quantity_drained) quantity_drained
from er
where
	er.remained > 0
	group by er.entry_type_id, er.code, er.description
	limit 3;`
	fmt.Println(sqlstr, cids)
	rows, err := tx.QueryContext(
		ctx,
		sqlstr,
		cids,
	)
	if err == sql.ErrNoRows {
		return nil, 0, dots.Errorf(dots.ENOTFOUND, "depletion for company not found")
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	cd := []*dots.CompanyDepletion{}
	for rows.Next() {
		var e dots.CompanyDepletion
		err := rows.Scan(&e.EntryTypeID, &e.Code, &e.Description, &e.QuantityInitial, &e.QuantityDrained)
		if err != nil {
			return nil, 0, err
		}
		cd = append(cd, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	n = len(cd)
	return cd, n, nil
}
