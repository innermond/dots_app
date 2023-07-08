package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

func quantityByEntryTypes(ctx context.Context, tx *Tx, etids []int) (map[int]float64, error) {
	sqlstr := `select entry_type_id, sum(quantity) from (
select e.date_added, e.id, e.entry_type_id, (e.quantity - coalesce((select sum(case when d.is_deleted = true then -d.quantity else d.quantity end)
from drain d
where d.entry_id = e.id), 0)
) quantity
from entry e
where e.entry_type_id = any($1)
) entrysync group by entry_type_id`

	rows, err := tx.QueryContext(ctx, sqlstr, etids)
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

func suggestDistributeOverEntryType(ctx context.Context, tx *Tx, etqty map[int]float64, strategy string) (map[int]float64, error) {
  switch strategy {
    case "new_many":
      strategy = "date_added desc, quantity desc"
    case "new_few":
      strategy = "date_added desc, quantity asc"
    case "old_many":
      strategy = "date_added asc, quantity desc"
    case "old_few":
      strategy = "date_added asc, quantity asc"
    default:
      strategy = "date_added desc, quantity desc"
  }
	// check if have enough quantities?
	var sqlb strings.Builder
	sqlb.WriteString(`with entrysync as (
 select e.date_added, e.id, e.entry_type_id, (e.quantity - coalesce((select sum(case when d.is_deleted = true then -d.quantity else d.quantity end)
from drain d
where d.entry_id = e.id), 0)
) quantity
from entry e
where e.entry_type_id = any($1)
), cumulative_sum as (
  select id, quantity, date_added, entry_type_id, SUM(quantity) over (partition by entry_type_id order by `+strategy+`, id) as running_sum
from entrysync where quantity > 0
)`)

	etids := []int{}
	inx := 0

	for etid, qty := range etqty {
		if inx > 0 {
			sqlb.WriteString("union all")
		}
		sqlb.WriteString(fmt.Sprintf(`
select id, case
  when running_sum <= %f then quantity
else quantity - (running_sum - %f)
  end as subtracted_quantity
from cumulative_sum
where entry_type_id = %d and quantity - (running_sum - %f) >= 0
`, qty, qty, etid, qty))
		inx++
		etids = append(etids, etid)
	}

	sqlstr := sqlb.String()
  fmt.Println(sqlstr)

	/*sqlstr := `
	with entrysync as (
	 select e.date_added, e.id, e.entry_type_id, (e.quantity - coalesce((select sum(case when d.is_deleted = true then -d.quantity else d.quantity end)
	from drain d
	where d.entry_id = e.id), 0)
	) quantity
	from entry e
	where e.entry_type_id = $1 and quantity > 0
	), cumulative_sum as (
	  select id, quantity, date_added, SUM(quantity) over (partition by entry_type_id order by date_added desc, quantity asc, id) as running_sum
	from entrysync
	)
	select id, case
	    when running_sum <= $2 then quantity
	else quantity - (running_sum - $2)
	  end as subtracted_quantity
	from cumulative_sum
	where quantity - (running_sum - $2) >= 0
		`
	*/

	rows, err := tx.QueryContext(ctx, sqlstr, etids)
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