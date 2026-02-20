package alpha

import (
	"strings"
	"testing"
)

const sampleCSV = `
"Legs · Day 2 · Week 4 · Push-Pull-Legs";"2026-02-19 4:54 h";"1:02 hr"
"1. Hack Squats · Machine · 8 reps";"WU1 · 37,5 kg · 9 reps<br>WU2 · 72,5 kg · 7 reps"
#;KG;REPS;RIR
1;115;8;1
2;115;10;1
3;115;10;1
"2. Sumo Squats · Smith machine · 10 reps";"WU1 · 35 kg · 8 reps"
#;KG;REPS;RIR
1;70;8;1
2;70;12;1
"3. Hyperextensions on Roman Chair · Bodyweight · 10 reps";"WU1 · +0 kg · 8 reps"
#;KG;REPS;RIR
1;+35;10;0
2;+35;9;1
3;+35;10;0
"4. Reverse Lunges · Dumbbells · 10 reps"
#;KG;REPS;RIR
1;10;10;1
2;10;10;1
3;10;10;0
"5. Standing Calf Raises · Machine · 12 reps";"WU1 · 47,5 kg · 8 reps"
#;KG;REPS;RIR
1;157,5;11;1
2;157,5;11;0
3;157,5;10;0
"6. Hanging Leg Raises · Bodyweight · 12 reps · 2 dropsets"
#;KG;REPS;RIR
1;+0;12;1
2;+0;12;1
3;+0;12;0

"Push · Day 1 · Week 4 · Push-Pull-Legs";"2026-02-17 5:04 h";"1:12 hr"
"1. Bench Press · Barbell · 6 reps";"WU1 · 22,5 kg · 10 reps<br>WU2 · 47,5 kg · 8 reps<br>WU3 · 77,5 kg · 6 reps"
#;KG;REPS;RIR
1;102,5;6;0
2;102,5;6;0
3;100;6;0
`

// TestParseCompleteSessions verifies parsing a multi-session CSV with exercises and sets.
// This is the primary integration test for the parser — covers the happy path end-to-end.
func TestParseCompleteSessions(t *testing.T) {
	sessions, err := Parse(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(sessions))
	}

	// First session — all 6 exercises
	s1 := sessions[0]
	if s1.Name != "Legs · Day 2 · Week 4 · Push-Pull-Legs" {
		t.Errorf("s1.Name = %q", s1.Name)
	}
	if s1.Duration != "1:02 hr" {
		t.Errorf("s1.Duration = %q", s1.Duration)
	}
	if len(s1.Exercises) != 6 {
		t.Fatalf("s1 exercises = %d, want 6", len(s1.Exercises))
	}

	// Exercise 1: Hack Squats — 2 warmups + 3 working sets, single-word equipment
	ex1 := s1.Exercises[0]
	if ex1.Name != "Hack Squats" {
		t.Errorf("ex1.Name = %q, want Hack Squats", ex1.Name)
	}
	if ex1.Equipment != "Machine" {
		t.Errorf("ex1.Equipment = %q, want Machine", ex1.Equipment)
	}
	if ex1.TargetReps != 8 {
		t.Errorf("ex1.TargetReps = %d, want 8", ex1.TargetReps)
	}
	if len(ex1.Sets) != 5 { // 2 warmup + 3 working
		t.Errorf("ex1 sets = %d, want 5", len(ex1.Sets))
	}

	// Exercise 2: Sumo Squats — multi-word equipment ("Smith machine")
	ex2 := s1.Exercises[1]
	if ex2.Name != "Sumo Squats" {
		t.Errorf("ex2.Name = %q, want Sumo Squats", ex2.Name)
	}
	if ex2.Equipment != "Smith machine" {
		t.Errorf("ex2.Equipment = %q, want Smith machine", ex2.Equipment)
	}
	if len(ex2.Sets) != 3 { // 1 warmup + 2 working
		t.Errorf("ex2 sets = %d, want 3", len(ex2.Sets))
	}

	// Exercise 3: Hyperextensions — multi-word name, bodyweight equipment
	ex3 := s1.Exercises[2]
	if ex3.Name != "Hyperextensions on Roman Chair" {
		t.Errorf("ex3.Name = %q, want Hyperextensions on Roman Chair", ex3.Name)
	}
	if ex3.Equipment != "Bodyweight" {
		t.Errorf("ex3.Equipment = %q, want Bodyweight", ex3.Equipment)
	}

	// Exercise 4: Reverse Lunges — no warmups
	ex4 := s1.Exercises[3]
	if ex4.Name != "Reverse Lunges" {
		t.Errorf("ex4.Name = %q, want Reverse Lunges", ex4.Name)
	}
	if ex4.Equipment != "Dumbbells" {
		t.Errorf("ex4.Equipment = %q, want Dumbbells", ex4.Equipment)
	}
	if len(ex4.Sets) != 3 { // 0 warmup + 3 working
		t.Errorf("ex4 sets = %d, want 3", len(ex4.Sets))
	}

	// Exercise 5: Standing Calf Raises — warmup with European decimal weight
	ex5 := s1.Exercises[4]
	if ex5.Name != "Standing Calf Raises" {
		t.Errorf("ex5.Name = %q, want Standing Calf Raises", ex5.Name)
	}
	if ex5.Equipment != "Machine" {
		t.Errorf("ex5.Equipment = %q, want Machine", ex5.Equipment)
	}
	if len(ex5.Sets) != 4 { // 1 warmup + 3 working
		t.Errorf("ex5 sets = %d, want 4", len(ex5.Sets))
	}

	// Exercise 6: Hanging Leg Raises — modifier "· 2 dropsets", no warmups, bodyweight
	ex6 := s1.Exercises[5]
	if ex6.Name != "Hanging Leg Raises" {
		t.Errorf("ex6.Name = %q, want Hanging Leg Raises", ex6.Name)
	}
	if ex6.Equipment != "Bodyweight" {
		t.Errorf("ex6.Equipment = %q, want Bodyweight", ex6.Equipment)
	}
	if ex6.TargetReps != 12 {
		t.Errorf("ex6.TargetReps = %d, want 12", ex6.TargetReps)
	}
	if len(ex6.Sets) != 3 { // 0 warmup + 3 working
		t.Errorf("ex6 sets = %d, want 3", len(ex6.Sets))
	}

	// Second session
	s2 := sessions[1]
	if s2.Name != "Push · Day 1 · Week 4 · Push-Pull-Legs" {
		t.Errorf("s2.Name = %q", s2.Name)
	}
}

// TestEuropeanDecimal verifies that European decimal notation is correctly parsed.
// Alpha Progression uses commas as decimal separators (e.g. "102,5" = 102.5 kg).
func TestEuropeanDecimal(t *testing.T) {
	got := parseEuropeanFloat("102,5")
	if got != 102.5 {
		t.Errorf("parseEuropeanFloat(102,5) = %f, want 102.5", got)
	}
}

// TestBodyweightPlus verifies the +N notation for bodyweight exercises.
// "+35" means bodyweight plus 35kg (e.g. weighted pullups).
func TestBodyweightPlus(t *testing.T) {
	weight, isBW := parseWeight("+35")
	if !isBW {
		t.Error("expected isBodyweightPlus=true")
	}
	if weight != 35 {
		t.Errorf("weight = %f, want 35", weight)
	}
}

// TestBodyweightPlusZero verifies that +0 means bodyweight only.
func TestBodyweightPlusZero(t *testing.T) {
	weight, isBW := parseWeight("+0")
	if !isBW {
		t.Error("expected isBodyweightPlus=true")
	}
	if weight != 0 {
		t.Errorf("weight = %f, want 0", weight)
	}
}

// TestFractionalRIR verifies that fractional RIR values are parsed correctly.
// Alpha Progression supports half-RIR values like "0,5".
func TestFractionalRIR(t *testing.T) {
	got := parseEuropeanFloat("0,5")
	if got != 0.5 {
		t.Errorf("parseEuropeanFloat(0,5) = %f, want 0.5", got)
	}
}

// TestWarmupParsing verifies warmup set extraction from the exercise header's second field.
// Warmups use <br> as separator and European decimal notation.
func TestWarmupParsing(t *testing.T) {
	warmupStr := "WU1 · 37,5 kg · 9 reps<br>WU2 · 72,5 kg · 7 reps"
	sets := parseWarmups(warmupStr)
	if len(sets) != 2 {
		t.Fatalf("warmup sets = %d, want 2", len(sets))
	}
	if sets[0].WeightKg != 37.5 {
		t.Errorf("wu1 weight = %f, want 37.5", sets[0].WeightKg)
	}
	if sets[0].Reps != 9 {
		t.Errorf("wu1 reps = %d, want 9", sets[0].Reps)
	}
	if !sets[0].IsWarmup {
		t.Error("wu1 should be warmup")
	}
	if sets[1].WeightKg != 72.5 {
		t.Errorf("wu2 weight = %f, want 72.5", sets[1].WeightKg)
	}
}

// TestWarmupBodyweightPlus verifies parsing warmup sets with bodyweight-plus notation.
func TestWarmupBodyweightPlus(t *testing.T) {
	warmupStr := "WU1 · +0 kg · 8 reps"
	sets := parseWarmups(warmupStr)
	if len(sets) != 1 {
		t.Fatalf("warmup sets = %d, want 1", len(sets))
	}
	if !sets[0].IsBodyweightPlus {
		t.Error("expected isBodyweightPlus=true")
	}
	if sets[0].WeightKg != 0 {
		t.Errorf("weight = %f, want 0", sets[0].WeightKg)
	}
}

// TestEmptyInput verifies that empty input returns no sessions without error.
func TestEmptyInput(t *testing.T) {
	sessions, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("sessions = %d, want 0", len(sessions))
	}
}

// TestSplitExerciseNameEquipment verifies name/equipment splitting from the combined field.
func TestSplitExerciseNameEquipment(t *testing.T) {
	name, equip := splitExerciseNameEquipment("Hack Squats · Machine")
	if name != "Hack Squats" {
		t.Errorf("name = %q", name)
	}
	if equip != "Machine" {
		t.Errorf("equip = %q", equip)
	}
}
