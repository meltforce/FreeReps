ALTER TABLE category_samples   DROP COLUMN IF EXISTS source_bundle;
ALTER TABLE sleep_stages       DROP COLUMN IF EXISTS source_bundle;
ALTER TABLE workout_heart_rate DROP COLUMN IF EXISTS source_bundle;
ALTER TABLE workouts           DROP COLUMN IF EXISTS source_bundle;
ALTER TABLE health_metrics     DROP COLUMN IF EXISTS source_bundle;
