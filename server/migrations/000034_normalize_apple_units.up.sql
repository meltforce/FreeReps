-- Health Auto Export delivers a metric in the unit its aggregation setting
-- implies: an aggregated export writes kcal and min, an export of individual
-- samples writes the HealthKit units kJ and s. On 2026-09-22 both forms stood
-- in health_metrics under one metric name. The aggregation in
-- internal/storage/health_metrics.go takes MAX(units) and adds qty up
-- regardless, so a range spanning the change summed kcal and kJ into one figure.
--
-- The ingest now converts to the canonical unit on write
-- (internal/ingest/health/units.go); this migration converts what is stored.
-- The conditions name the unit, so a second run finds no rows.

-- time_in_daylight had no display unit at all, which is why nothing declared
-- which of the two forms was meant.
UPDATE metric_allowlist SET display_unit = 'min' WHERE metric_name = 'time_in_daylight';

UPDATE health_metrics SET qty = qty / 4.184, units = 'kcal'
WHERE metric_name IN ('active_energy', 'basal_energy_burned') AND units = 'kJ';

UPDATE health_metrics SET qty = qty / 60, units = 'min'
WHERE metric_name = 'time_in_daylight' AND units = 's';

-- Same quantity under a different name, so the value stays.
UPDATE health_metrics SET units = 'bpm'
WHERE metric_name = 'walking_heart_rate_average' AND units = 'count/min';
