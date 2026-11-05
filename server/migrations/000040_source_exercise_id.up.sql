-- Key the exercise map by a source-side exercise id where the source has one.
--
-- openGym delivers a catalogue id per logged exercise ("0025") and no name, and
-- a custom exercise keeps its id while the user renames it in the app. A map
-- keyed by name would key on a value FreeReps writes itself. The id is stored
-- per set in workout_sets.source_exercise_id; the map's key column becomes
-- source_key and holds that id for openGym and the exercise name for Alpha
-- Progression, whose rows leave source_exercise_id empty. Every reader joins on
-- COALESCE(NULLIF(ws.source_exercise_id, ''), ws.exercise_name).
-- DECISIONS.md, 2026-11-05.
--
-- Adding a column with a constant default rewrites no rows (PostgreSQL 11+).

ALTER TABLE workout_sets ADD COLUMN source_exercise_id TEXT NOT NULL DEFAULT '';

ALTER TABLE exercise_name_map RENAME COLUMN exercise_name TO source_key;

-- Seed: the 23 openGym catalogue ids in the operator's history, each mapped
-- onto the Hevy template FreeReps stores for the same sessions (reviewed
-- 2026-11-05 against workout_sets rows with source = 'Hevy'; table and
-- reasoning in server/specs/opengym-api.md). These rows are data written for
-- FreeReps, not taken from openGym's own Hevy import map.
--
-- 1326 chin-up maps to Chin Up (Weighted): 26 of the 29 chin-up sets in the
-- history carry added load, and the unweighted Chin Up template has the same
-- muscle groups, so only the name and the body-weight flag depend on the choice.
--
-- 0572 lever assisted chin-up maps to Chin Up (Assisted). Its weight is
-- assistance, not load; the ingest refuses a bodyweight_assisted exercise until
-- that case is decided (open question in the spec), and the mapping is what
-- lets it recognise one.
--
-- The WHERE EXISTS guard has the same reason as in 000023: on a fresh instance
-- exercise_templates is empty until the first Hevy sync.
INSERT INTO exercise_name_map (source, source_key, exercise_template_id, note)
SELECT 'openGym', v.id, v.template_id, ''
FROM (VALUES
    ('0025', '79D0BB3A'),  -- barbell bench press → Bench Press (Barbell)
    ('0031', 'A5AC6449'),  -- barbell curl → Bicep Curl (Barbell)
    ('0047', '50DFDFAB'),  -- barbell incline bench press → Incline Bench Press (Barbell)
    ('0150', '6A6C31A5'),  -- cable bar lateral pulldown → Lat Pulldown (Cable)
    ('0192', 'DE68C825'),  -- cable one arm lateral raise → Single Arm Lateral Raise (Cable)
    ('0201', '93A552C6'),  -- cable pushdown → Triceps Pushdown
    ('0277', 'B2398CD1'),  -- decline crunch → Decline Crunch (Weighted)
    ('0334', '422B08F1'),  -- dumbbell lateral raise → Lateral Raise (Dumbbell)
    ('0383', 'B582299E'),  -- dumbbell reverse fly → Chest Supported Reverse Fly (Dumbbell)
    ('0410', 'B5D3A742'),  -- dumbbell single leg split squat → Bulgarian Split Squat (Dumbbell)
    ('0447', '01A35BF9'),  -- ez barbell curl → EZ Bar Biceps Curl
    ('0472', 'F8356514'),  -- hanging leg raise → Hanging Leg Raise
    ('0572', 'D23C609B'),  -- lever assisted chin-up → Chin Up (Assisted)
    ('0586', 'B8127AD1'),  -- lever lying leg curl → Lying Leg Curl (Machine)
    ('0602', 'D8281C62'),  -- lever seated reverse fly → Rear Delt Reverse Fly (Machine)
    ('0605', 'E05C2C38'),  -- lever standing calf raise → Standing Calf Raise (Machine)
    ('0652', '729237D1'),  -- pull-up → Pull Up (Weighted)
    ('0739', 'C7973E0E'),  -- sled 45° leg press → Leg Press (Machine)
    ('1326', '023943F1'),  -- chin-up → Chin Up (Weighted)
    ('1454', '9237BAD1'),  -- lever seated shoulder press → Seated Shoulder Press (Machine)
    ('1755', '10347BAC'),  -- weighted tricep dips → Triceps Dip (Weighted)
    ('2611', '0EB695C9'),  -- lever horizontal one leg press → Leg Press Horizontal (Machine)
    ('2661', '0393F233'),  -- cable seated row with v bar → Seated Cable Row - V Grip (Cable)
    ('5035', '091737FA')   -- weighted hyperextension → Back Extension (Weighted Hyperextension)
) AS v(id, template_id)
WHERE EXISTS (SELECT 1 FROM exercise_templates t WHERE t.id = v.template_id)
ON CONFLICT (source, source_key) DO NOTHING;
