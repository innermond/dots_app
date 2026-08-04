create or replace view api.entry_with_quantity_drained as 
select 
  e.id, e.entry_type_id, e.date_added, e.company_id,
  e.quantity quantity_initial,
  (
    select
      coalesce(sum(case when d.is_deleted = true then 0 else d.quantity end), 0)
    from core.drain d
    where d.entry_id = e.id
  ) quantity_drained
from api.entry e;
