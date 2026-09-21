-- Removes what the up migration derived, identified by the workout's source
-- and the row's own source. The summary returns to NULL, which is the value an
-- Oura workout row carries when the mapper writes it.
DELETE FROM workout_heart_rate hr
USING workouts w
WHERE hr.workout_id = w.id
  AND hr.user_id = w.user_id
  AND hr.source = 'Oura'
  AND w.source = 'Oura';

UPDATE workouts SET
  avg_heart_rate = NULL,
  min_heart_rate = NULL,
  max_heart_rate = NULL
WHERE source = 'Oura';
