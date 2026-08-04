create trigger company_has_same_tid_tg before insert or update on core.deed for each row execute function core.company_has_same_tid();
create trigger deed_entry_has_same_tid_tg before insert or update on core.drain for each row execute function core.deed_entry_same_tid();
create trigger entry_type_company_has_same_tid_tg before insert or update on core.entry for each row execute function core.entry_type_company_same_tid();
