package ingest

// workoutNameMap normalizes workout names from various sources to canonical English names.
// Handles German translations (Apple Health), Indoor/Outdoor prefixes, and Oura lowercase names.
// The is_indoor field on the workout already captures indoor/outdoor distinction.
var workoutNameMap = map[string]string{
	// German (Apple Health localized)
	"Abkühlen":                         "Cooldown",
	"Flexibilität":                     "Flexibility",
	"Freiwasser Schwimmen":             "Swimming",
	"Funktionales Krafttraining":       "Functional Strength Training",
	"Hochintensives Intervalltraining": "High Intensity Interval Training",
	"Innenräume Radfahren":             "Cycling",
	"Innenräume Spaziergang":           "Walking",
	"Kerntraining":                     "Core Training",
	"Outdoor Ausführen":                "Running",
	"Outdoor Radfahren":                "Cycling",
	"Outdoor Spaziergang":              "Walking",
	"Rudern":                           "Rowing",
	"Schwimmbad Schwimmen":             "Swimming",
	"Sonstige":                         "Other",
	"Traditionelles Krafttraining":     "Traditional Strength Training",
	"Wandern":                          "Hiking",

	// Apple Health (English, location-prefixed)
	"Indoor Cycling": "Cycling",
	"Outdoor Walk":   "Walking",
	"Outdoor Run":    "Running",
	"Indoor Run":     "Running",
	"Outdoor Cycle":  "Cycling",

	// Oura (lowercase activity names)
	"walking":                    "Walking",
	"running":                    "Running",
	"cycling":                    "Cycling",
	"yoga":                       "Yoga",
	"hiking":                     "Hiking",
	"swimming":                   "Swimming",
	"rowing":                     "Rowing",
	"other":                      "Other",
	"strength_training":          "Strength Training",
	"hiit":                       "High Intensity Interval Training",
	"pilates":                    "Pilates",
	"dancing":                    "Dancing",
	"elliptical":                 "Elliptical",
	"stair_climbing":             "Stair Climbing",
	"cross_training":             "Cross Training",
	"flexibility":                "Flexibility",
	"cooldown":                   "Cooldown",
	"core_training":              "Core Training",
	"indoor_cycling":             "Cycling",
	"outdoor_cycling":            "Cycling",
	"outdoor_running":            "Running",
	"indoor_running":             "Running",
	"traditional_strength_training": "Traditional Strength Training",
	"functional_strength_training":  "Functional Strength Training",
}

// NormalizeWorkoutName maps a raw workout name to a canonical English name.
// Returns the input unchanged if no mapping exists.
func NormalizeWorkoutName(name string) string {
	if normalized, ok := workoutNameMap[name]; ok {
		return normalized
	}
	return name
}
