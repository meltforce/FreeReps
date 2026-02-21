-- Remove DEFAULT 1 from all user_id columns.
-- Every code path already sets user_id explicitly. Removing the default ensures
-- that any future bug that forgets to set user_id causes a NOT NULL violation
-- instead of silently assigning data to user 1.

ALTER TABLE health_metrics     ALTER COLUMN user_id DROP DEFAULT;
ALTER TABLE sleep_sessions     ALTER COLUMN user_id DROP DEFAULT;
ALTER TABLE sleep_stages       ALTER COLUMN user_id DROP DEFAULT;
ALTER TABLE workouts           ALTER COLUMN user_id DROP DEFAULT;
ALTER TABLE workout_heart_rate ALTER COLUMN user_id DROP DEFAULT;
ALTER TABLE workout_routes     ALTER COLUMN user_id DROP DEFAULT;
ALTER TABLE workout_sets       ALTER COLUMN user_id DROP DEFAULT;
