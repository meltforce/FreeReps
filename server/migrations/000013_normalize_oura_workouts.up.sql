-- Normalize Oura lowercase workout names to match canonical English names.
-- Also set source = 'Oura' on workouts that came from Oura (identified by
-- raw_json containing oura-specific fields).
UPDATE workouts SET name = 'Walking' WHERE name = 'walking';
UPDATE workouts SET name = 'Running' WHERE name = 'running';
UPDATE workouts SET name = 'Cycling' WHERE name = 'cycling';
UPDATE workouts SET name = 'Yoga' WHERE name = 'yoga';
UPDATE workouts SET name = 'Hiking' WHERE name = 'hiking';
UPDATE workouts SET name = 'Swimming' WHERE name = 'swimming';
UPDATE workouts SET name = 'Other' WHERE name = 'other';
UPDATE workouts SET name = 'Strength Training' WHERE name = 'strength_training';
UPDATE workouts SET name = 'High Intensity Interval Training' WHERE name = 'hiit';
UPDATE workouts SET name = 'Flexibility' WHERE name = 'flexibility';
UPDATE workouts SET name = 'Cooldown' WHERE name = 'cooldown';
UPDATE workouts SET name = 'Core Training' WHERE name = 'core_training';
UPDATE workouts SET name = 'Cycling' WHERE name = 'indoor_cycling';
UPDATE workouts SET name = 'Cycling' WHERE name = 'outdoor_cycling';
UPDATE workouts SET name = 'Running' WHERE name = 'indoor_running';
UPDATE workouts SET name = 'Running' WHERE name = 'outdoor_running';
UPDATE workouts SET name = 'Traditional Strength Training' WHERE name = 'traditional_strength_training';
UPDATE workouts SET name = 'Functional Strength Training' WHERE name = 'functional_strength_training';

-- Tag Oura workouts with source. Oura workouts have raw_json containing "activity" key
-- (Oura API field), while Apple Watch workouts don't.
UPDATE workouts SET source = 'Oura' WHERE source = '' AND raw_json::text LIKE '%"activity"%' AND raw_json::text LIKE '%"intensity"%';
