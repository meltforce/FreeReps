-- Best-effort reversal of German workout name normalization.
-- Cannot perfectly reverse many-to-one mappings (e.g., multiple German names → "Cycling").
-- This restores the most common original name for each mapping.

-- Note: "Cycling" was mapped from both "Innenräume Radfahren" and "Outdoor Radfahren"
-- and "Indoor Cycling". We cannot distinguish them after normalization, so this is a no-op
-- for ambiguous cases.

-- Unambiguous reversals only:
UPDATE workouts SET name = 'Abkühlen' WHERE name = 'Cooldown';
UPDATE workouts SET name = 'Flexibilität' WHERE name = 'Flexibility';
UPDATE workouts SET name = 'Funktionales Krafttraining' WHERE name = 'Functional Strength Training';
UPDATE workouts SET name = 'Hochintensives Intervalltraining' WHERE name = 'High Intensity Interval Training';
UPDATE workouts SET name = 'Kerntraining' WHERE name = 'Core Training';
UPDATE workouts SET name = 'Rudern' WHERE name = 'Rowing';
UPDATE workouts SET name = 'Sonstige' WHERE name = 'Other';
UPDATE workouts SET name = 'Traditionelles Krafttraining' WHERE name = 'Traditional Strength Training';
UPDATE workouts SET name = 'Wandern' WHERE name = 'Hiking';
