create extension if not exists pgcrypto;

create type flow_intensity as enum ('light', 'medium', 'heavy');
create type reminder_type as enum ('period_upcoming', 'fertile_window', 'medication', 'checkup');
create type sharing_status as enum ('pending', 'active', 'revoked');

create or replace function set_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;

-- profiles

create table profiles (
  id uuid primary key references auth.users(id) on delete cascade,
  display_name text,
  birth_year int,
  default_cycle_length int not null default 28,
  default_period_length int not null default 5,
  preferences jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create trigger trg_profiles_updated_at
  before update on profiles
  for each row execute function set_updated_at();

-- cycles

create table cycles (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  start_date date not null,
  end_date date,
  cycle_length int,
  period_length int,
  is_outlier boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (user_id, start_date)
);

create trigger trg_cycles_updated_at
  before update on cycles
  for each row execute function set_updated_at();

create index idx_cycles_user_start_date on cycles (user_id, start_date);

-- symptoms

create table symptoms (
  id uuid primary key default gen_random_uuid(),
  user_id uuid references auth.users(id) on delete cascade,
  name text not null,
  category text,
  is_custom boolean not null default false,
  created_at timestamptz not null default now()
);

-- daily_logs

create table daily_logs (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  cycle_id uuid references cycles(id) on delete set null,
  date date not null,
  flow_intensity flow_intensity,
  mood text,
  notes text check (char_length(notes) <= 500),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (user_id, date)
);

create trigger trg_daily_logs_updated_at
  before update on daily_logs
  for each row execute function set_updated_at();

create index idx_daily_logs_user_date on daily_logs (user_id, date);

-- daily_log_symptoms (junction table)

create table daily_log_symptoms (
  daily_log_id uuid not null references daily_logs(id) on delete cascade,
  symptom_id uuid not null references symptoms(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (daily_log_id, symptom_id)
);

-- wellness_logs

create table wellness_logs (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  date date not null,
  water_glasses int,
  sleep_hours numeric(3, 1),
  weight_kg numeric(5, 2),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (user_id, date)
);

create trigger trg_wellness_logs_updated_at
  before update on wellness_logs
  for each row execute function set_updated_at();

create index idx_wellness_logs_user_date on wellness_logs (user_id, date);

-- reminders

create table reminders (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  type reminder_type not null,
  is_enabled boolean not null default true,
  time_of_day time,
  days_before int,
  custom_message text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create trigger trg_reminders_updated_at
  before update on reminders
  for each row execute function set_updated_at();

-- sharing_permissions (disiapkan untuk v2, belum dipakai di v1)

create table sharing_permissions (
  id uuid primary key default gen_random_uuid(),
  owner_user_id uuid not null references auth.users(id) on delete cascade,
  partner_user_id uuid references auth.users(id) on delete cascade,
  invite_code text not null unique,
  shared_categories jsonb not null default '[]'::jsonb,
  status sharing_status not null default 'pending',
  expires_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create trigger trg_sharing_permissions_updated_at
  before update on sharing_permissions
  for each row execute function set_updated_at();
