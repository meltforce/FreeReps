-- ============================================================
-- source_uuid on health_metrics for HealthBeat UUID dedup
-- ============================================================
ALTER TABLE health_metrics ADD COLUMN IF NOT EXISTS source_uuid UUID;

CREATE INDEX IF NOT EXISTS idx_health_metrics_source_uuid
    ON health_metrics (source_uuid) WHERE source_uuid IS NOT NULL;

-- ============================================================
-- ecg_recordings: ECG waveforms from Apple Watch
-- ============================================================
CREATE TABLE ecg_recordings (
    id                   UUID             PRIMARY KEY,
    user_id              INTEGER          NOT NULL,
    classification       TEXT             NOT NULL,
    average_heart_rate   DOUBLE PRECISION,
    sampling_frequency   DOUBLE PRECISION,
    voltage_measurements JSONB,
    start_date           TIMESTAMPTZ      NOT NULL,
    source               TEXT             NOT NULL DEFAULT ''
);

CREATE INDEX idx_ecg_recordings_user_date ON ecg_recordings (user_id, start_date DESC);

-- ============================================================
-- audiograms: hearing test results
-- ============================================================
CREATE TABLE audiograms (
    id                  UUID             PRIMARY KEY,
    user_id             INTEGER          NOT NULL,
    sensitivity_points  JSONB            NOT NULL,
    start_date          TIMESTAMPTZ      NOT NULL,
    source              TEXT             NOT NULL DEFAULT ''
);

CREATE INDEX idx_audiograms_user_date ON audiograms (user_id, start_date DESC);

-- ============================================================
-- activity_summaries: daily Apple Watch rings
-- ============================================================
CREATE TABLE activity_summaries (
    user_id              INTEGER          NOT NULL,
    date                 DATE             NOT NULL,
    active_energy        DOUBLE PRECISION,
    active_energy_goal   DOUBLE PRECISION,
    exercise_time        DOUBLE PRECISION,
    exercise_time_goal   DOUBLE PRECISION,
    stand_hours          DOUBLE PRECISION,
    stand_hours_goal     DOUBLE PRECISION,
    PRIMARY KEY (user_id, date)
);

-- ============================================================
-- medications: dose events
-- ============================================================
CREATE TABLE medications (
    id              UUID             PRIMARY KEY,
    user_id         INTEGER          NOT NULL,
    name            TEXT             NOT NULL,
    dosage          TEXT,
    log_status      TEXT,
    start_date      TIMESTAMPTZ      NOT NULL,
    end_date        TIMESTAMPTZ,
    source          TEXT             NOT NULL DEFAULT ''
);

CREATE INDEX idx_medications_user_date ON medications (user_id, start_date DESC);

-- ============================================================
-- vision_prescriptions: glasses/contacts Rx
-- ============================================================
CREATE TABLE vision_prescriptions (
    id                UUID             PRIMARY KEY,
    user_id           INTEGER          NOT NULL,
    date_issued       TIMESTAMPTZ      NOT NULL,
    expiration_date   TIMESTAMPTZ,
    prescription_type TEXT,
    right_eye         JSONB,
    left_eye          JSONB,
    source            TEXT             NOT NULL DEFAULT ''
);

CREATE INDEX idx_vision_prescriptions_user_date ON vision_prescriptions (user_id, date_issued DESC);

-- ============================================================
-- state_of_mind: iOS 18+ mood/emotion logging
-- ============================================================
CREATE TABLE state_of_mind (
    id              UUID             PRIMARY KEY,
    user_id         INTEGER          NOT NULL,
    kind            INTEGER          NOT NULL,
    valence         DOUBLE PRECISION NOT NULL,
    labels          INTEGER[],
    associations    INTEGER[],
    start_date      TIMESTAMPTZ      NOT NULL,
    source          TEXT             NOT NULL DEFAULT ''
);

CREATE INDEX idx_state_of_mind_user_date ON state_of_mind (user_id, start_date DESC);

-- ============================================================
-- category_samples: HKCategorySample records (sleep stages, symptoms, etc.)
-- ============================================================
CREATE TABLE category_samples (
    id              UUID             PRIMARY KEY,
    user_id         INTEGER          NOT NULL,
    type            TEXT             NOT NULL,
    value           INTEGER          NOT NULL,
    value_label     TEXT,
    start_date      TIMESTAMPTZ      NOT NULL,
    end_date        TIMESTAMPTZ      NOT NULL,
    source          TEXT             NOT NULL DEFAULT ''
);

CREATE INDEX idx_category_samples_user_type_date ON category_samples (user_id, type, start_date DESC);

-- ============================================================
-- Seed metric_allowlist with all HealthKit quantity types
-- Uses ON CONFLICT DO NOTHING to avoid duplicating V1 seed data.
-- ============================================================
INSERT INTO metric_allowlist (metric_name, category) VALUES
    -- Activity (some already exist from V1)
    ('step_count',                          'activity'),
    ('distance_walking_running',            'activity'),
    ('distance_cycling',                    'activity'),
    ('distance_swimming',                   'activity'),
    ('distance_wheelchair',                 'activity'),
    ('basal_energy_burned',                 'activity'),
    ('active_energy',                       'activity'),
    ('flights_climbed',                     'activity'),
    ('apple_exercise_time',                 'activity'),
    ('apple_move_time',                     'activity'),
    ('apple_stand_time',                    'activity'),
    ('push_count',                          'activity'),
    ('swimming_stroke_count',               'activity'),
    ('distance_downhill_snow_sports',       'activity'),
    ('time_in_daylight',                    'activity'),
    ('physical_effort',                     'activity'),
    ('estimated_workout_effort_score',      'activity'),
    ('workout_effort_score',                'activity'),
    -- Body
    ('weight_body_mass',                    'body'),
    ('body_mass_index',                     'body'),
    ('body_fat_percentage',                 'body'),
    ('height',                              'body'),
    ('lean_body_mass',                      'body'),
    ('waist_circumference',                 'body'),
    ('apple_sleeping_wrist_temperature',    'body'),
    -- Vitals
    ('heart_rate',                          'cardiovascular'),
    ('resting_heart_rate',                  'cardiovascular'),
    ('walking_heart_rate_average',          'cardiovascular'),
    ('heart_rate_variability',              'cardiovascular'),
    ('blood_oxygen_saturation',             'cardiovascular'),
    ('body_temperature',                    'cardiovascular'),
    ('basal_body_temperature',              'cardiovascular'),
    ('respiratory_rate',                    'cardiovascular'),
    ('blood_pressure_systolic',             'cardiovascular'),
    ('blood_pressure_diastolic',            'cardiovascular'),
    ('peripheral_perfusion_index',          'cardiovascular'),
    ('electrodermal_activity',              'cardiovascular'),
    ('heart_rate_recovery_one_minute',      'cardiovascular'),
    ('atrial_fibrillation_burden',          'cardiovascular'),
    -- Mobility & Fitness
    ('vo2_max',                             'fitness'),
    ('walking_speed',                       'fitness'),
    ('walking_step_length',                 'fitness'),
    ('walking_asymmetry_percentage',        'fitness'),
    ('walking_double_support_percentage',   'fitness'),
    ('stair_ascent_speed',                  'fitness'),
    ('stair_descent_speed',                 'fitness'),
    ('six_minute_walk_test_distance',       'fitness'),
    ('running_stride_length',              'fitness'),
    ('running_vertical_oscillation',       'fitness'),
    ('running_ground_contact_time',        'fitness'),
    ('running_power',                      'fitness'),
    ('running_speed',                      'fitness'),
    ('apple_walking_steadiness',           'fitness'),
    ('cycling_speed',                      'fitness'),
    ('cycling_power',                      'fitness'),
    ('cycling_functional_threshold_power', 'fitness'),
    ('cycling_cadence',                    'fitness'),
    -- Lab & Clinical
    ('blood_glucose',                       'lab'),
    ('insulin_delivery',                    'lab'),
    ('blood_alcohol_content',               'lab'),
    ('number_of_times_fallen',              'lab'),
    -- Hearing
    ('environmental_audio_exposure',        'hearing'),
    ('headphone_audio_exposure',            'hearing'),
    -- Respiratory
    ('forced_vital_capacity',               'respiratory'),
    ('forced_expiratory_volume_1',          'respiratory'),
    ('peak_expiratory_flow_rate',           'respiratory'),
    ('inhaler_usage',                       'respiratory'),
    ('uv_exposure',                         'other'),
    -- Nutrition
    ('dietary_energy_consumed',             'nutrition'),
    ('dietary_protein',                     'nutrition'),
    ('dietary_carbohydrates',               'nutrition'),
    ('dietary_fat_total',                   'nutrition'),
    ('dietary_fat_saturated',               'nutrition'),
    ('dietary_fat_monounsaturated',         'nutrition'),
    ('dietary_fat_polyunsaturated',         'nutrition'),
    ('dietary_sugar',                       'nutrition'),
    ('dietary_fiber',                       'nutrition'),
    ('dietary_cholesterol',                 'nutrition'),
    ('dietary_sodium',                      'nutrition'),
    ('dietary_calcium',                     'nutrition'),
    ('dietary_phosphorus',                  'nutrition'),
    ('dietary_magnesium',                   'nutrition'),
    ('dietary_potassium',                   'nutrition'),
    ('dietary_iron',                        'nutrition'),
    ('dietary_zinc',                        'nutrition'),
    ('dietary_manganese',                   'nutrition'),
    ('dietary_copper',                      'nutrition'),
    ('dietary_selenium',                    'nutrition'),
    ('dietary_chromium',                    'nutrition'),
    ('dietary_molybdenum',                  'nutrition'),
    ('dietary_iodine',                      'nutrition'),
    ('dietary_vitamin_a',                   'nutrition'),
    ('dietary_vitamin_b6',                  'nutrition'),
    ('dietary_vitamin_b12',                 'nutrition'),
    ('dietary_vitamin_c',                   'nutrition'),
    ('dietary_vitamin_d',                   'nutrition'),
    ('dietary_vitamin_e',                   'nutrition'),
    ('dietary_vitamin_k',                   'nutrition'),
    ('dietary_thiamin',                     'nutrition'),
    ('dietary_riboflavin',                  'nutrition'),
    ('dietary_niacin',                      'nutrition'),
    ('dietary_pantothenic_acid',            'nutrition'),
    ('dietary_folate',                      'nutrition'),
    ('dietary_biotin',                      'nutrition'),
    ('dietary_caffeine',                    'nutrition'),
    ('dietary_water',                       'nutrition'),
    ('dietary_chloride',                    'nutrition'),
    -- Sleep
    ('sleep_analysis',                      'sleep'),
    -- Other
    ('underwater_depth',                    'other'),
    ('water_temperature',                   'other')
ON CONFLICT (metric_name) DO NOTHING;
