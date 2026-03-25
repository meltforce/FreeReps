-- Add source column to workouts for multi-source dedup.
ALTER TABLE workouts ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';
