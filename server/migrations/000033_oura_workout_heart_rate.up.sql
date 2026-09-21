-- Oura workouts carry no heart rate: the API's workout object has no such
-- field, while /v2/usercollection/heartrate reports the samples of the same
-- interval with a source of their own. The sync now derives the series for
-- every workout it writes (storage.FillWorkoutHeartRateFromMetrics); this
-- migration does the same for the Oura workouts already stored, from the
-- heart_rate metrics the sync has kept all along.
--
-- Minutes, not samples: an average over samples is an average of the sampling
-- rate as much as of the heart rate, because Oura records a 5-second burst
-- while a workout is running and returns to its sparse sampling afterwards.
-- Measured on 2026-09-21 for a 103-minute session, 77 of whose 92 samples sit
-- in the first six minutes: 98.32 over the samples, 96.16 over the 16 minutes.
INSERT INTO workout_heart_rate (time, workout_id, user_id, min_bpm, avg_bpm, max_bpm, source)
SELECT date_trunc('minute', m.time), w.id, w.user_id,
       MIN(COALESCE(m.min_val, m.qty)),
       AVG(COALESCE(m.avg_val, m.qty)),
       MAX(COALESCE(m.max_val, m.qty)),
       'Oura'
FROM workouts w
JOIN health_metrics m
  ON m.user_id = w.user_id
 AND m.metric_name = 'heart_rate'
 AND m.source = 'Oura'
 AND m.time >= w.start_time
 AND m.time < w.end_time
WHERE w.source = 'Oura'
  AND COALESCE(m.avg_val, m.qty) IS NOT NULL
GROUP BY w.id, w.user_id, 1
ON CONFLICT DO NOTHING;

-- Only rows that have no summary yet. The mapper writes none for an Oura
-- workout, so this is every one of them; the condition keeps a figure that
-- some other path may have written.
UPDATE workouts w SET
  avg_heart_rate = s.avg_bpm,
  min_heart_rate = s.min_bpm,
  max_heart_rate = s.max_bpm
FROM (SELECT workout_id, user_id,
             AVG(avg_bpm) AS avg_bpm, MIN(min_bpm) AS min_bpm, MAX(max_bpm) AS max_bpm
      FROM workout_heart_rate
      GROUP BY workout_id, user_id) s
WHERE w.id = s.workout_id
  AND w.user_id = s.user_id
  AND w.source = 'Oura'
  AND w.avg_heart_rate IS NULL;
