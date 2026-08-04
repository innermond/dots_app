create or replace view api.company with (security_invoker=true) as 
select id, longname, tin, rn
from core.company
where deleted_at is null;

create or replace view api.deed with (security_invoker=true) as 
select id, company_id, title, quantity, unit, unitprice
from core.deed
where deleted_at is null;

create or replace view api.entry with (security_invoker=true) as 
select id, entry_type_id, date_added, quantity, company_id
from core.entry
where deleted_at is null;

create or replace view api.entry_type with (security_invoker=true) as 
select id, code, description, unit
from core.entry_type
where deleted_at is null;
