delete from symptoms where user_id is null and is_custom = false;
drop index if exists idx_symptoms_preset_name;
