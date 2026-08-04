package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/innermond/dots"
)

var (
	ErrDistributeEmpty       = errors.New("empty distribute")
	ErrDistributeNotEnough   = errors.New("not enough quantities")
	ErrDistributeCalculation = errors.New("not found a solution")
)

// TODO this is no longer used as will force calling code to use a map that
// will alter the order given from database
func DistributeFrom(dd map[int]map[int]float64, etd map[int]float64) (map[int]float64, error) {

	if len(dd) == 0 {
		return nil, ErrDistributeEmpty
	}

	distribute := map[int]map[int]float64{}
	for etid, requiredOty := range etd {
		idqty, found := dd[etid]
		if !found {
			continue
		}
		entryOty := map[int]float64{}
		for id, qty := range idqty {
			if requiredOty == 0.0 {
				continue
			}
			// enough case
			if requiredOty <= qty {
				entryOty[id] = requiredOty
				// it is completly consumed
				// this consuming state will be checked later in code
				requiredOty = 0
				break
			}
			// not enough need more entries to consume
			requiredOty -= qty
			entryOty[id] = qty
		}
		distribute[etid] = entryOty
		// hasn't been consumed
		if requiredOty > 0 {
			return nil, ErrDistributeNotEnough
		}
	}

	// found no solution
	if len(distribute) == 0 {
		return nil, ErrDistributeEmpty
	}

	calculated := map[int]float64{}
	for _, eidqty := range distribute {
		for eid, qty := range eidqty {
			calculated[eid] = qty
		}
	}

	if len(calculated) == 0 {
		return nil, ErrDistributeCalculation
	}

	return calculated, nil
}

func quantityByEntryTypes(ctx context.Context, tx *Tx, etids []int, cid int) (map[int]float64, error) {
	sqlstr := `select entry_type_id, sum(quantity_initial - quantity_drained) quantity
from entry_with_quantity_drained e
where e.entry_type_id = any($1) and e.company_id = $2
group by e.entry_type_id`

	rows, err := tx.QueryContext(ctx, sqlstr, etids, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[int]float64{}
	for rows.Next() {
		var (
			eid int
			qty float64
		)
		err = rows.Scan(&eid, &qty)
		if err != nil {
			return nil, err
		}
		m[eid] = qty
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, sql.ErrNoRows
	}

	return m, nil
}

func quantityByEntries(ctx context.Context, tx *Tx, eids []int, cid int) (map[int]float64, error) {
	sqlstr := `select e.id, (quantity_initial - quantity_drained) quantity
from entry_with_quantity_drained e
where e.id = any($1) and e.company_id = $2
`

	rows, err := tx.QueryContext(ctx, sqlstr, eids, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[int]float64{}
	for rows.Next() {
		var (
			eid int
			qty float64
		)
		err = rows.Scan(&eid, &qty)
		if err != nil {
			return nil, err
		}
		m[eid] = qty
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, sql.ErrNoRows
	}

	return m, nil
}

func distributeOverEntryType(ctx context.Context, tx *Tx, etqty map[int]float64, cid int, strategy string) (map[int]float64, error) {
	switch strategy {
	case "new_many":
		strategy = "date_added desc, quantity desc"
	case "new_few":
		strategy = "date_added desc, quantity asc"
	case "old_many":
		strategy = "date_added asc, quantity desc"
	case "old_few":
		strategy = "date_added asc, quantity asc"
	case "many_new":
		strategy = "quantity desc, date_added desc"
	case "few_new":
		strategy = "quantity asc, date_added desc"
	case "many_old":
		strategy = "quantity desc, date_added asc"
	case "few_old":
		strategy = "quantity asc, date_added asc"
	default:
		strategy = "date_added desc, quantity desc"
	}
	// check if have enough quantities?
	var sqlb strings.Builder
	sqlb.WriteString(`select id, subtracted_quantity from (
with
wanted (etid, qty) as (
	values `)
	tpl := "(%d,%f)"
	values := []string{}
	for etid, qty := range etqty {
		values = append(values, fmt.Sprintf(tpl, etid, qty))
	}
	sqlb.WriteString(strings.Join(values, ","))
	sqlb.WriteString(`
),
`)
	sqlb.WriteString(`entrysync as (
select
	e.*,
	(e.quantity_initial - coalesce(quantity_drained, 0)) quantity
from entry_with_quantity_drained e
where e.entry_type_id = any($1)
and e.company_id = $2),
`)
	sqlb.WriteString(`cumulative_sum as (
   select
   (select sum(qty) from wanted where etid = es.entry_type_id group by etid) wqty,
   id, quantity, date_added, entry_type_id,
   SUM(quantity) over (partition by entry_type_id order by ` + strategy + `, id) as running_sum
from entrysync es
where quantity > 0
)
`)
	sqlb.WriteString(`select
	id,
	case
		when running_sum <= cs.wqty then quantity
		else quantity - (running_sum - cs.wqty)
	end as subtracted_quantity
from cumulative_sum cs
) dist
where dist.subtracted_quantity >= 0;`)

	sqlstr := sqlb.String()
	fmt.Println(sqlstr)

	etids := keysOf(etqty)

	rows, err := tx.QueryContext(ctx, sqlstr, etids, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[int]float64{}
	for rows.Next() {
		var (
			eid int
			qty float64
		)
		err = rows.Scan(&eid, &qty)
		if err != nil {
			return nil, err
		}
		m[eid] = aprox(qty, 5)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, sql.ErrNoRows
	}

	return m, nil
}

func tryDistributeOverEntryType(ctx context.Context, tx *Tx, etqty map[int]float64, cid int, strategy string) (map[int]float64, error) {
	etids := keysOf(etqty)
	etqtyExistent, err := quantityByEntryTypes(ctx, tx, etids, cid)
	if err != nil {
		return nil, err
	}

	notexistent := []string{}
	needmore := map[int]float64{}
	for k, wanted := range etqty {
		if existent, found := etqtyExistent[k]; !found {
			notexistent = append(notexistent, fmt.Sprintf("%v", k))
		} else if wanted > existent {
			needmore[k] = wanted - existent
		}
	}

	numNotfound := len(notexistent)
	if numNotfound > 0 {
		d := map[string]interface{}{"notfound": notexistent}
		return nil, dots.Errorf(dots.ENOTFOUND, "not found entry type %d", numNotfound).WithData(d)
	}

	if len(needmore) > 0 {
		err := dots.Errorf(dots.EINVALID, "not enough quantity")
		err.Data = map[string]interface{}{"needmore": needmore}
		return nil, err
	}

	distribute, err := distributeOverEntryType(ctx, tx, etqty, cid, strategy)
	if err != nil {
		return nil, err
	}

	return distribute, nil
}
