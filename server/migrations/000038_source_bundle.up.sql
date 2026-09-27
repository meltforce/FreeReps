-- The HealthKit bundle identifier of the app or device that wrote a row, as the
-- iOS app reports it; '' for rows from every other path and for rows stored
-- before the app sent it.
--
-- source holds the name the source priority ranks, derived from this column by
-- health.CanonicalSource: every Apple device and app maps to '' ("Apple
-- Health"), a provider FreeReps also syncs directly maps to that provider's
-- name. The bundle is kept next to it so a later change to that mapping can be
-- traced against the rows it was applied to.
ALTER TABLE health_metrics     ADD COLUMN source_bundle TEXT NOT NULL DEFAULT '';
ALTER TABLE workouts           ADD COLUMN source_bundle TEXT NOT NULL DEFAULT '';
ALTER TABLE workout_heart_rate ADD COLUMN source_bundle TEXT NOT NULL DEFAULT '';
ALTER TABLE sleep_stages       ADD COLUMN source_bundle TEXT NOT NULL DEFAULT '';
ALTER TABLE category_samples   ADD COLUMN source_bundle TEXT NOT NULL DEFAULT '';
