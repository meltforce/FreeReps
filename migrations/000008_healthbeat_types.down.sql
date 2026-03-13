DROP TABLE IF EXISTS category_samples;
DROP TABLE IF EXISTS state_of_mind;
DROP TABLE IF EXISTS vision_prescriptions;
DROP TABLE IF EXISTS medications;
DROP TABLE IF EXISTS activity_summaries;
DROP TABLE IF EXISTS audiograms;
DROP TABLE IF EXISTS ecg_recordings;

ALTER TABLE health_metrics DROP COLUMN IF EXISTS source_uuid;

-- Remove allowlist entries added by this migration.
-- Keep the original V1 entries (heart_rate, resting_heart_rate, etc.).
DELETE FROM metric_allowlist WHERE metric_name IN (
    'step_count', 'distance_walking_running', 'distance_cycling', 'distance_swimming',
    'distance_wheelchair', 'flights_climbed', 'apple_move_time', 'apple_stand_time',
    'push_count', 'swimming_stroke_count', 'distance_downhill_snow_sports',
    'time_in_daylight', 'physical_effort', 'estimated_workout_effort_score', 'workout_effort_score',
    'body_mass_index', 'height', 'lean_body_mass', 'waist_circumference',
    'walking_heart_rate_average', 'body_temperature', 'basal_body_temperature',
    'blood_pressure_systolic', 'blood_pressure_diastolic', 'peripheral_perfusion_index',
    'electrodermal_activity', 'heart_rate_recovery_one_minute', 'atrial_fibrillation_burden',
    'walking_speed', 'walking_step_length', 'walking_asymmetry_percentage',
    'walking_double_support_percentage', 'stair_ascent_speed', 'stair_descent_speed',
    'six_minute_walk_test_distance', 'running_stride_length', 'running_vertical_oscillation',
    'running_ground_contact_time', 'running_power', 'running_speed', 'apple_walking_steadiness',
    'cycling_speed', 'cycling_power', 'cycling_functional_threshold_power', 'cycling_cadence',
    'blood_glucose', 'insulin_delivery', 'blood_alcohol_content', 'number_of_times_fallen',
    'environmental_audio_exposure', 'headphone_audio_exposure',
    'forced_vital_capacity', 'forced_expiratory_volume_1', 'peak_expiratory_flow_rate',
    'inhaler_usage', 'uv_exposure',
    'dietary_energy_consumed', 'dietary_protein', 'dietary_carbohydrates', 'dietary_fat_total',
    'dietary_fat_saturated', 'dietary_fat_monounsaturated', 'dietary_fat_polyunsaturated',
    'dietary_sugar', 'dietary_fiber', 'dietary_cholesterol', 'dietary_sodium',
    'dietary_calcium', 'dietary_phosphorus', 'dietary_magnesium', 'dietary_potassium',
    'dietary_iron', 'dietary_zinc', 'dietary_manganese', 'dietary_copper', 'dietary_selenium',
    'dietary_chromium', 'dietary_molybdenum', 'dietary_iodine',
    'dietary_vitamin_a', 'dietary_vitamin_b6', 'dietary_vitamin_b12', 'dietary_vitamin_c',
    'dietary_vitamin_d', 'dietary_vitamin_e', 'dietary_vitamin_k',
    'dietary_thiamin', 'dietary_riboflavin', 'dietary_niacin', 'dietary_pantothenic_acid',
    'dietary_folate', 'dietary_biotin', 'dietary_caffeine', 'dietary_water', 'dietary_chloride',
    'underwater_depth', 'water_temperature'
);
