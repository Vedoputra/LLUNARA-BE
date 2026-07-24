drop policy if exists "owner_or_partner_can_select_shares" on sharing_permissions;
drop policy if exists "owner_can_insert_own_shares" on sharing_permissions;
drop policy if exists "owner_can_update_own_shares" on sharing_permissions;
drop policy if exists "owner_can_delete_own_shares" on sharing_permissions;
alter table sharing_permissions disable row level security;

drop policy if exists "user_can_select_own_reminders" on reminders;
drop policy if exists "user_can_insert_own_reminders" on reminders;
drop policy if exists "user_can_update_own_reminders" on reminders;
drop policy if exists "user_can_delete_own_reminders" on reminders;
alter table reminders disable row level security;

drop policy if exists "user_can_select_own_wellness_logs" on wellness_logs;
drop policy if exists "user_can_insert_own_wellness_logs" on wellness_logs;
drop policy if exists "user_can_update_own_wellness_logs" on wellness_logs;
drop policy if exists "user_can_delete_own_wellness_logs" on wellness_logs;
alter table wellness_logs disable row level security;

drop policy if exists "user_can_select_own_daily_log_symptoms" on daily_log_symptoms;
drop policy if exists "user_can_insert_own_daily_log_symptoms" on daily_log_symptoms;
drop policy if exists "user_can_delete_own_daily_log_symptoms" on daily_log_symptoms;
alter table daily_log_symptoms disable row level security;

drop policy if exists "user_can_select_own_daily_logs" on daily_logs;
drop policy if exists "user_can_insert_own_daily_logs" on daily_logs;
drop policy if exists "user_can_update_own_daily_logs" on daily_logs;
drop policy if exists "user_can_delete_own_daily_logs" on daily_logs;
alter table daily_logs disable row level security;

drop policy if exists "read_presets_and_own_symptoms" on symptoms;
drop policy if exists "user_can_insert_own_symptoms" on symptoms;
drop policy if exists "user_can_update_own_symptoms" on symptoms;
drop policy if exists "user_can_delete_own_symptoms" on symptoms;
alter table symptoms disable row level security;

drop policy if exists "user_can_select_own_cycles" on cycles;
drop policy if exists "user_can_insert_own_cycles" on cycles;
drop policy if exists "user_can_update_own_cycles" on cycles;
drop policy if exists "user_can_delete_own_cycles" on cycles;
alter table cycles disable row level security;

drop policy if exists "user_can_select_own_profile" on profiles;
drop policy if exists "user_can_insert_own_profile" on profiles;
drop policy if exists "user_can_update_own_profile" on profiles;
drop policy if exists "user_can_delete_own_profile" on profiles;
alter table profiles disable row level security;
