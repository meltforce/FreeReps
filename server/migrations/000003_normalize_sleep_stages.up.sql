-- Normalize localized sleep stage names to canonical English.
-- Covers German, French, Spanish, Italian (most common non-English locales).
-- Uses NOT EXISTS guard to avoid unique constraint violations when the
-- canonical name already exists for the same (start_time, end_time, user_id).

-- German
UPDATE sleep_stages SET stage = 'Core'  WHERE stage = 'Kern'    AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Core');
UPDATE sleep_stages SET stage = 'Deep'  WHERE stage = 'Tief'    AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Deep');
UPDATE sleep_stages SET stage = 'Awake' WHERE stage = 'Wach'    AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Awake');
UPDATE sleep_stages SET stage = 'In Bed' WHERE stage = 'Im Bett' AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'In Bed');

-- French
UPDATE sleep_stages SET stage = 'REM'   WHERE stage = 'Paradoxal' AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'REM');
UPDATE sleep_stages SET stage = 'Deep'  WHERE stage = 'Profond'   AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Deep');
UPDATE sleep_stages SET stage = 'Core'  WHERE stage = 'Léger'     AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Core');
UPDATE sleep_stages SET stage = 'Awake' WHERE stage = 'Éveillé'   AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Awake');
UPDATE sleep_stages SET stage = 'In Bed' WHERE stage = 'Au lit'   AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'In Bed');

-- Spanish
UPDATE sleep_stages SET stage = 'Deep'  WHERE stage = 'Profundo'   AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Deep');
UPDATE sleep_stages SET stage = 'Core'  WHERE stage = 'Principal'  AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Core');
UPDATE sleep_stages SET stage = 'Awake' WHERE stage = 'Despierto'  AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Awake');
UPDATE sleep_stages SET stage = 'Awake' WHERE stage = 'Despierta'  AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Awake');
UPDATE sleep_stages SET stage = 'In Bed' WHERE stage = 'En la cama' AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'In Bed');

-- Italian
UPDATE sleep_stages SET stage = 'Deep'  WHERE stage = 'Profondo'    AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Deep');
UPDATE sleep_stages SET stage = 'Core'  WHERE stage = 'Essenziale'  AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Core');
UPDATE sleep_stages SET stage = 'Awake' WHERE stage = 'Sveglio'     AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'Awake');
UPDATE sleep_stages SET stage = 'In Bed' WHERE stage = 'A letto'    AND NOT EXISTS (SELECT 1 FROM sleep_stages s2 WHERE s2.start_time = sleep_stages.start_time AND s2.end_time = sleep_stages.end_time AND s2.user_id = sleep_stages.user_id AND s2.stage = 'In Bed');

-- Delete broken zero-value sessions from previous backfill attempts.
-- These have total_sleep=0 because the backfill couldn't match localized stage names.
DELETE FROM sleep_sessions WHERE total_sleep = 0 AND deep = 0 AND core = 0 AND rem = 0;

-- Delete corresponding zero-value sleep_analysis health metrics from backfill.
DELETE FROM health_metrics WHERE metric_name = 'sleep_analysis' AND source = 'FreeReps Backfill' AND qty = 0;
