-- SpO2 is stored as fraction (0-1) by Apple Health. Display as percentage.
UPDATE metric_allowlist SET display_multiplier = 100 WHERE metric_name = 'blood_oxygen_saturation';

-- Fix existing Oura respiratory rate data that was incorrectly multiplied by 60.
-- Oura average_breath is already breaths/minute, not breaths/second.
UPDATE health_metrics SET qty = qty / 60 WHERE metric_name = 'respiratory_rate' AND source = 'Oura' AND qty > 100;

-- Fix existing Oura SpO2 data stored as percentage (should be fraction for consistency).
UPDATE health_metrics SET qty = qty / 100 WHERE metric_name = 'blood_oxygen_saturation' AND source = 'Oura' AND qty > 1;
