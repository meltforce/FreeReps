ALTER TABLE health_metrics     ALTER COLUMN user_id SET DEFAULT 1;
ALTER TABLE sleep_sessions     ALTER COLUMN user_id SET DEFAULT 1;
ALTER TABLE sleep_stages       ALTER COLUMN user_id SET DEFAULT 1;
ALTER TABLE workouts           ALTER COLUMN user_id SET DEFAULT 1;
ALTER TABLE workout_heart_rate ALTER COLUMN user_id SET DEFAULT 1;
ALTER TABLE workout_routes     ALTER COLUMN user_id SET DEFAULT 1;
ALTER TABLE workout_sets       ALTER COLUMN user_id SET DEFAULT 1;
