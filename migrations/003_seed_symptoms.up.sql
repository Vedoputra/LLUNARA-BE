-- Prevent duplicate system presets on repeated runs, without restricting
-- custom per-user tags from sharing a name with each other.
create unique index if not exists idx_symptoms_preset_name on symptoms (name) where user_id is null;

insert into symptoms (user_id, name, category, is_custom) values
  (null, 'kram', 'physical', false),
  (null, 'sakit kepala', 'physical', false),
  (null, 'nyeri payudara', 'physical', false),
  (null, 'kembung', 'physical', false),
  (null, 'jerawat', 'physical', false),
  (null, 'kelelahan', 'physical', false),
  (null, 'nyeri punggung', 'physical', false),
  (null, 'mual', 'physical', false),
  (null, 'perubahan nafsu makan', 'physical', false),
  (null, 'sulit tidur', 'physical', false),
  (null, 'keputihan', 'physical', false)
on conflict (name) where user_id is null do nothing;
