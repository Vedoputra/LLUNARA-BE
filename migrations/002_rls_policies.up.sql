-- profiles (id IS the user id, references auth.users.id directly)

alter table profiles enable row level security;

create policy "user_can_select_own_profile" on profiles for select using (auth.uid() = id);
create policy "user_can_insert_own_profile" on profiles for insert with check (auth.uid() = id);
create policy "user_can_update_own_profile" on profiles for update using (auth.uid() = id);
create policy "user_can_delete_own_profile" on profiles for delete using (auth.uid() = id);

-- cycles

alter table cycles enable row level security;

create policy "user_can_select_own_cycles" on cycles for select using (auth.uid() = user_id);
create policy "user_can_insert_own_cycles" on cycles for insert with check (auth.uid() = user_id);
create policy "user_can_update_own_cycles" on cycles for update using (auth.uid() = user_id);
create policy "user_can_delete_own_cycles" on cycles for delete using (auth.uid() = user_id);

-- symptoms (user_id null = system preset, readable by everyone, only owner can modify their own custom tags)

alter table symptoms enable row level security;

create policy "read_presets_and_own_symptoms" on symptoms for select
  using (user_id is null or auth.uid() = user_id);
create policy "user_can_insert_own_symptoms" on symptoms for insert with check (auth.uid() = user_id);
create policy "user_can_update_own_symptoms" on symptoms for update using (auth.uid() = user_id);
create policy "user_can_delete_own_symptoms" on symptoms for delete using (auth.uid() = user_id);

-- daily_logs

alter table daily_logs enable row level security;

create policy "user_can_select_own_daily_logs" on daily_logs for select using (auth.uid() = user_id);
create policy "user_can_insert_own_daily_logs" on daily_logs for insert with check (auth.uid() = user_id);
create policy "user_can_update_own_daily_logs" on daily_logs for update using (auth.uid() = user_id);
create policy "user_can_delete_own_daily_logs" on daily_logs for delete using (auth.uid() = user_id);

-- daily_log_symptoms (no user_id column — ownership checked via join to daily_logs)

alter table daily_log_symptoms enable row level security;

create policy "user_can_select_own_daily_log_symptoms" on daily_log_symptoms for select
  using (exists (
    select 1 from daily_logs dl
    where dl.id = daily_log_symptoms.daily_log_id and dl.user_id = auth.uid()
  ));

create policy "user_can_insert_own_daily_log_symptoms" on daily_log_symptoms for insert
  with check (exists (
    select 1 from daily_logs dl
    where dl.id = daily_log_symptoms.daily_log_id and dl.user_id = auth.uid()
  ));

create policy "user_can_delete_own_daily_log_symptoms" on daily_log_symptoms for delete
  using (exists (
    select 1 from daily_logs dl
    where dl.id = daily_log_symptoms.daily_log_id and dl.user_id = auth.uid()
  ));

-- wellness_logs

alter table wellness_logs enable row level security;

create policy "user_can_select_own_wellness_logs" on wellness_logs for select using (auth.uid() = user_id);
create policy "user_can_insert_own_wellness_logs" on wellness_logs for insert with check (auth.uid() = user_id);
create policy "user_can_update_own_wellness_logs" on wellness_logs for update using (auth.uid() = user_id);
create policy "user_can_delete_own_wellness_logs" on wellness_logs for delete using (auth.uid() = user_id);

-- reminders

alter table reminders enable row level security;

create policy "user_can_select_own_reminders" on reminders for select using (auth.uid() = user_id);
create policy "user_can_insert_own_reminders" on reminders for insert with check (auth.uid() = user_id);
create policy "user_can_update_own_reminders" on reminders for update using (auth.uid() = user_id);
create policy "user_can_delete_own_reminders" on reminders for delete using (auth.uid() = user_id);

-- sharing_permissions (v2, unused in v1 — owner has full control, partner can only read shares directed at them)

alter table sharing_permissions enable row level security;

create policy "owner_or_partner_can_select_shares" on sharing_permissions for select
  using (auth.uid() = owner_user_id or auth.uid() = partner_user_id);
create policy "owner_can_insert_own_shares" on sharing_permissions for insert with check (auth.uid() = owner_user_id);
create policy "owner_can_update_own_shares" on sharing_permissions for update using (auth.uid() = owner_user_id);
create policy "owner_can_delete_own_shares" on sharing_permissions for delete using (auth.uid() = owner_user_id);
