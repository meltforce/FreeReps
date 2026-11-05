DELETE FROM exercise_name_map WHERE source = 'openGym';
ALTER TABLE exercise_name_map RENAME COLUMN source_key TO exercise_name;
ALTER TABLE workout_sets DROP COLUMN IF EXISTS source_exercise_id;
