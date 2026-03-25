-- Add display metadata to metric_allowlist for server-driven UI.
ALTER TABLE metric_allowlist ADD COLUMN IF NOT EXISTS display_label TEXT NOT NULL DEFAULT '';
ALTER TABLE metric_allowlist ADD COLUMN IF NOT EXISTS display_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE metric_allowlist ADD COLUMN IF NOT EXISTS is_cumulative BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE metric_allowlist ADD COLUMN IF NOT EXISTS display_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1.0;

-- Cardiovascular
UPDATE metric_allowlist SET display_label = 'Heart Rate', display_unit = 'bpm' WHERE metric_name = 'heart_rate';
UPDATE metric_allowlist SET display_label = 'Resting HR', display_unit = 'bpm' WHERE metric_name = 'resting_heart_rate';
UPDATE metric_allowlist SET display_label = 'Walking HR Avg', display_unit = 'bpm' WHERE metric_name = 'walking_heart_rate_average';
UPDATE metric_allowlist SET display_label = 'HRV', display_unit = 'ms' WHERE metric_name = 'heart_rate_variability';
UPDATE metric_allowlist SET display_label = 'SpO2', display_unit = '%' WHERE metric_name = 'blood_oxygen_saturation';
UPDATE metric_allowlist SET display_label = 'Resp. Rate', display_unit = 'brpm' WHERE metric_name = 'respiratory_rate';
UPDATE metric_allowlist SET display_label = 'Body Temp', display_unit = '°C' WHERE metric_name = 'body_temperature';
UPDATE metric_allowlist SET display_label = 'BP Systolic', display_unit = 'mmHg' WHERE metric_name = 'blood_pressure_systolic';
UPDATE metric_allowlist SET display_label = 'BP Diastolic', display_unit = 'mmHg' WHERE metric_name = 'blood_pressure_diastolic';

-- Fitness
UPDATE metric_allowlist SET display_label = 'VO2 Max', display_unit = 'mL/kg/min' WHERE metric_name = 'vo2_max';
UPDATE metric_allowlist SET display_label = 'Walking Speed', display_unit = 'm/s' WHERE metric_name = 'walking_speed';
UPDATE metric_allowlist SET display_label = 'Walking Step Length', display_unit = 'm' WHERE metric_name = 'walking_step_length';
UPDATE metric_allowlist SET display_label = 'Running Speed', display_unit = 'm/s' WHERE metric_name = 'running_speed';
UPDATE metric_allowlist SET display_label = 'Running Stride', display_unit = 'm' WHERE metric_name = 'running_stride_length';
UPDATE metric_allowlist SET display_label = 'Cycling Speed', display_unit = 'm/s' WHERE metric_name = 'cycling_speed';
UPDATE metric_allowlist SET display_label = 'Cycling Cadence', display_unit = 'rpm' WHERE metric_name = 'cycling_cadence';

-- Body
UPDATE metric_allowlist SET display_label = 'Weight', display_unit = 'kg' WHERE metric_name = 'weight_body_mass';
UPDATE metric_allowlist SET display_label = 'BMI', display_unit = 'kg/m²' WHERE metric_name = 'body_mass_index';
UPDATE metric_allowlist SET display_label = 'Body Fat', display_unit = '%', display_multiplier = 100 WHERE metric_name = 'body_fat_percentage';
UPDATE metric_allowlist SET display_label = 'Lean Body Mass', display_unit = 'kg' WHERE metric_name = 'lean_body_mass';
UPDATE metric_allowlist SET display_label = 'Height', display_unit = 'cm' WHERE metric_name = 'height';

-- Activity (cumulative)
UPDATE metric_allowlist SET display_label = 'Active Energy', display_unit = 'kcal', is_cumulative = true WHERE metric_name = 'active_energy';
UPDATE metric_allowlist SET display_label = 'Basal Energy', display_unit = 'kcal', is_cumulative = true WHERE metric_name = 'basal_energy_burned';
UPDATE metric_allowlist SET display_label = 'Exercise Time', display_unit = 'min', is_cumulative = true WHERE metric_name = 'apple_exercise_time';
UPDATE metric_allowlist SET display_label = 'Steps', display_unit = 'count', is_cumulative = true WHERE metric_name = 'step_count';
UPDATE metric_allowlist SET display_label = 'Walking + Running', display_unit = 'km', is_cumulative = true WHERE metric_name = 'distance_walking_running';
UPDATE metric_allowlist SET display_label = 'Cycling Distance', display_unit = 'km', is_cumulative = true WHERE metric_name = 'distance_cycling';
UPDATE metric_allowlist SET display_label = 'Swimming Distance', display_unit = 'm', is_cumulative = true WHERE metric_name = 'distance_swimming';
UPDATE metric_allowlist SET display_label = 'Flights Climbed', display_unit = 'count', is_cumulative = true WHERE metric_name = 'flights_climbed';
UPDATE metric_allowlist SET display_label = 'Stand Time', display_unit = 'min', is_cumulative = true WHERE metric_name = 'apple_stand_time';
UPDATE metric_allowlist SET display_label = 'Move Time', display_unit = 'min', is_cumulative = true WHERE metric_name = 'apple_move_time';

-- Sleep
UPDATE metric_allowlist SET display_label = 'Sleep Duration', display_unit = 'hr' WHERE metric_name = 'sleep_analysis';
UPDATE metric_allowlist SET display_label = 'Wrist Temp', display_unit = '°C' WHERE metric_name = 'apple_sleeping_wrist_temperature';

-- Hearing
UPDATE metric_allowlist SET display_label = 'Env. Audio', display_unit = 'dB' WHERE metric_name = 'environmental_audio_exposure';
UPDATE metric_allowlist SET display_label = 'Headphone Audio', display_unit = 'dB' WHERE metric_name = 'headphone_audio_exposure';

-- Oura-exclusive
UPDATE metric_allowlist SET display_label = 'Readiness Score', display_unit = 'score' WHERE metric_name = 'oura_readiness_score';
UPDATE metric_allowlist SET display_label = 'Sleep Score', display_unit = 'score' WHERE metric_name = 'oura_sleep_score';
UPDATE metric_allowlist SET display_label = 'Activity Score', display_unit = 'score' WHERE metric_name = 'oura_activity_score';
UPDATE metric_allowlist SET display_label = 'Temp. Deviation', display_unit = '°C' WHERE metric_name = 'oura_temperature_deviation';
UPDATE metric_allowlist SET display_label = 'Stress (High)', display_unit = 's' WHERE metric_name = 'oura_stress_high';
UPDATE metric_allowlist SET display_label = 'Recovery (High)', display_unit = 's' WHERE metric_name = 'oura_recovery_high';
UPDATE metric_allowlist SET display_label = 'Resilience', display_unit = 'level' WHERE metric_name = 'oura_resilience';
UPDATE metric_allowlist SET display_label = 'Cardiovascular Age', display_unit = 'years' WHERE metric_name = 'oura_cardiovascular_age';
