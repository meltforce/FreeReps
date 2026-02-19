package alpha

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
)

var (
	// sessionHeaderRe matches: "Session Name";"2026-02-19 4:54 h";"1:02 hr"
	sessionHeaderRe = regexp.MustCompile(`^"(.+)";"(\d{4}-\d{2}-\d{2}\s+\d+:\d+)\s+h";"(.+)"$`)

	// exerciseHeaderRe matches: "1. Exercise Name · Equipment · 8 reps[· modifiers]"[;"warmup info"]
	exerciseHeaderRe = regexp.MustCompile(`^"(\d+)\.\s+(.+?)(?:\s+·\s+(\S.*?))?\s+·\s+(\d+)\s+reps(.*?)"(?:;"(.+)")?$`)

	// setDataRe matches: 1;115;8;1
	setDataRe = regexp.MustCompile(`^(\d+);(.+);(\d+);(.+)$`)

	// warmupRe matches: WU1 · 37,5 kg · 9 reps
	warmupRe = regexp.MustCompile(`WU(\d+)\s+·\s+(.+?)\s+kg\s+·\s+(\d+)\s+reps`)

	// columnHeaderRe matches: #;KG;REPS;RIR
	columnHeaderRe = regexp.MustCompile(`^#;KG;REPS;RIR$`)
)

// Parse reads an Alpha Progression CSV export and returns parsed sessions.
func Parse(r io.Reader) ([]models.AlphaSession, error) {
	scanner := bufio.NewScanner(r)
	var sessions []models.AlphaSession
	var current *models.AlphaSession
	var currentExercise *models.AlphaExercise

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Blank line = session boundary
		if line == "" {
			if current != nil {
				if currentExercise != nil {
					current.Exercises = append(current.Exercises, *currentExercise)
					currentExercise = nil
				}
				sessions = append(sessions, *current)
				current = nil
			}
			continue
		}

		// Skip column headers
		if columnHeaderRe.MatchString(line) {
			continue
		}

		// Try session header
		if m := sessionHeaderRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				if currentExercise != nil {
					current.Exercises = append(current.Exercises, *currentExercise)
					currentExercise = nil
				}
				sessions = append(sessions, *current)
			}
			date, err := parseSessionDate(m[2])
			if err != nil {
				return nil, fmt.Errorf("parsing session date %q: %w", m[2], err)
			}
			current = &models.AlphaSession{
				Name:     m[1],
				Date:     date,
				Duration: m[3],
			}
			continue
		}

		// Try exercise header
		if m := exerciseHeaderRe.FindStringSubmatch(line); m != nil {
			if current == nil {
				return nil, fmt.Errorf("exercise without session: %q", line)
			}
			if currentExercise != nil {
				current.Exercises = append(current.Exercises, *currentExercise)
			}
			num, _ := strconv.Atoi(m[1])
			targetReps, _ := strconv.Atoi(m[4])

			// m[2] = exercise name, m[3] = equipment (optional, captured by regex)
			currentExercise = &models.AlphaExercise{
				Number:     num,
				Name:       strings.TrimSpace(m[2]),
				Equipment:  strings.TrimSpace(m[3]),
				TargetReps: targetReps,
			}

			// Parse warmup sets if present
			if m[6] != "" {
				warmups := parseWarmups(m[6])
				currentExercise.Sets = append(currentExercise.Sets, warmups...)
			}
			continue
		}

		// Try set data
		if m := setDataRe.FindStringSubmatch(line); m != nil {
			if currentExercise == nil {
				return nil, fmt.Errorf("set data without exercise: %q", line)
			}
			setNum, _ := strconv.Atoi(m[1])
			weight, isBW := parseWeight(m[2])
			reps, _ := strconv.Atoi(m[3])
			rir := parseEuropeanFloat(m[4])

			currentExercise.Sets = append(currentExercise.Sets, models.AlphaSet{
				Number:           setNum,
				WeightKg:         weight,
				IsBodyweightPlus: isBW,
				Reps:             reps,
				RIR:              rir,
				IsWarmup:         false,
			})
			continue
		}

		// Unknown line — skip silently (could be notes or other metadata)
	}

	// Flush remaining
	if current != nil {
		if currentExercise != nil {
			current.Exercises = append(current.Exercises, *currentExercise)
		}
		sessions = append(sessions, *current)
	}

	return sessions, scanner.Err()
}

// parseSessionDate parses "2026-02-19 4:54" into a time.Time.
func parseSessionDate(s string) (time.Time, error) {
	// Try both formats: "2026-02-19 4:54" and "2026-02-19 16:54"
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02 3:04"} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}

// splitExerciseNameEquipment splits "Hack Squats · Machine" into name and equipment.
func splitExerciseNameEquipment(s string) (name, equipment string) {
	parts := strings.Split(s, " · ")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[len(parts)-1])
	}
	return strings.TrimSpace(s), ""
}

// parseWarmups extracts warmup sets from the warmup info string.
// Example: "WU1 · 37,5 kg · 9 reps<br>WU2 · 72,5 kg · 7 reps"
func parseWarmups(s string) []models.AlphaSet {
	var sets []models.AlphaSet
	parts := strings.Split(s, "<br>")
	for _, part := range parts {
		m := warmupRe.FindStringSubmatch(part)
		if m == nil {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		weight, isBW := parseWeight(m[2])
		reps, _ := strconv.Atoi(m[3])
		sets = append(sets, models.AlphaSet{
			Number:           num,
			WeightKg:         weight,
			IsBodyweightPlus: isBW,
			Reps:             reps,
			IsWarmup:         true,
		})
	}
	return sets
}

// parseWeight handles European decimals and bodyweight-plus notation.
// "+35" -> (35, true), "102,5" -> (102.5, false), "+0" -> (0, true)
func parseWeight(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "+") {
		w := parseEuropeanFloat(s[1:])
		return w, true
	}
	return parseEuropeanFloat(s), false
}

// parseEuropeanFloat converts a European decimal string to float64.
// "102,5" -> 102.5, "0,5" -> 0.5
func parseEuropeanFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
