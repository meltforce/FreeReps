-- Normalize German workout names from Health Auto Export to English equivalents.
-- The is_indoor flag already captures indoor/outdoor distinction, so location prefixes
-- are stripped from both German and English names.

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
UPDATE workouts SET name = 'Cycling' WHERE name = 'Indoor Cycling';
UPDATE workouts SET name = 'Walking' WHERE name = 'Outdoor Walk';
