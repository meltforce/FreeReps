-- Normalize Apple Health location-prefixed and German workout names in existing data.
-- These mappings match the shared NormalizeWorkoutName() function.

-- English location-prefixed
UPDATE workouts SET name = 'Walking' WHERE name = 'Outdoor Walk';
UPDATE workouts SET name = 'Running' WHERE name = 'Outdoor Run';
UPDATE workouts SET name = 'Cycling' WHERE name = 'Indoor Cycling';
UPDATE workouts SET name = 'Cycling' WHERE name = 'Outdoor Cycle';
UPDATE workouts SET name = 'Running' WHERE name = 'Indoor Run';

-- German
UPDATE workouts SET name = 'Cooldown' WHERE name = 'Abkühlen';
UPDATE workouts SET name = 'Flexibility' WHERE name = 'Flexibilität';
UPDATE workouts SET name = 'Swimming' WHERE name = 'Freiwasser Schwimmen';
UPDATE workouts SET name = 'Functional Strength Training' WHERE name = 'Funktionales Krafttraining';
UPDATE workouts SET name = 'High Intensity Interval Training' WHERE name = 'Hochintensives Intervalltraining';
UPDATE workouts SET name = 'Cycling' WHERE name = 'Innenräume Radfahren';
UPDATE workouts SET name = 'Walking' WHERE name = 'Innenräume Spaziergang';
UPDATE workouts SET name = 'Core Training' WHERE name = 'Kerntraining';
UPDATE workouts SET name = 'Running' WHERE name = 'Outdoor Ausführen';
UPDATE workouts SET name = 'Cycling' WHERE name = 'Outdoor Radfahren';
UPDATE workouts SET name = 'Walking' WHERE name = 'Outdoor Spaziergang';
UPDATE workouts SET name = 'Rowing' WHERE name = 'Rudern';
UPDATE workouts SET name = 'Swimming' WHERE name = 'Schwimmbad Schwimmen';
UPDATE workouts SET name = 'Other' WHERE name = 'Sonstige';
UPDATE workouts SET name = 'Traditional Strength Training' WHERE name = 'Traditionelles Krafttraining';
UPDATE workouts SET name = 'Hiking' WHERE name = 'Wandern';
