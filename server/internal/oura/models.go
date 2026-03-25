package oura

// Oura API v2 response types. Generated from docs/oura/openapi-1.28.json.
// All collection endpoints return Response[T] with pagination via NextToken.

// Response is the pagination wrapper for all Oura API v2 collection endpoints.
type Response[T any] struct {
	Data      []T     `json:"data"`
	NextToken *string `json:"next_token"`
}

// DailyReadinessItem from GET /v2/usercollection/daily_readiness.
type DailyReadinessItem struct {
	ID                        string   `json:"id"`
	Day                       string   `json:"day"`
	Score                     *int     `json:"score"`
	TemperatureDeviation      *float64 `json:"temperature_deviation"`
	TemperatureTrendDeviation *float64 `json:"temperature_trend_deviation"`
	Timestamp                 string   `json:"timestamp"`
}

// DailySleepItem from GET /v2/usercollection/daily_sleep.
type DailySleepItem struct {
	ID        string `json:"id"`
	Day       string `json:"day"`
	Score     *int   `json:"score"`
	Timestamp string `json:"timestamp"`
}

// DailyActivityItem from GET /v2/usercollection/daily_activity.
type DailyActivityItem struct {
	ID                        string `json:"id"`
	Day                       string `json:"day"`
	Score                     *int   `json:"score"`
	ActiveCalories            int    `json:"active_calories"`
	Steps                     int    `json:"steps"`
	EquivalentWalkingDistance int    `json:"equivalent_walking_distance"`
	TotalCalories             int    `json:"total_calories"`
	Timestamp                 string `json:"timestamp"`
}

// SleepItem from GET /v2/usercollection/sleep.
type SleepItem struct {
	ID                 string   `json:"id"`
	Day                string   `json:"day"`
	BedtimeStart       string   `json:"bedtime_start"`
	BedtimeEnd         string   `json:"bedtime_end"`
	DeepSleepDuration  *int     `json:"deep_sleep_duration"`
	LightSleepDuration *int     `json:"light_sleep_duration"`
	REMSleepDuration   *int     `json:"rem_sleep_duration"`
	TotalSleepDuration *int     `json:"total_sleep_duration"`
	AwakeTime          *int     `json:"awake_time"`
	TimeInBed          int      `json:"time_in_bed"`
	AverageHeartRate   *float64 `json:"average_heart_rate"`
	AverageHRV         *int     `json:"average_hrv"`
	LowestHeartRate    *int     `json:"lowest_heart_rate"`
	AverageBreath      *float64 `json:"average_breath"`
	SleepPhase5Min     *string  `json:"sleep_phase_5_min"`
	Efficiency         *int     `json:"efficiency"`
	Latency            *int     `json:"latency"`
	Type               string   `json:"type"`
}

// HeartRateItem from GET /v2/usercollection/heartrate.
type HeartRateItem struct {
	BPM       int    `json:"bpm"`
	Source    string `json:"source"`
	Timestamp string `json:"timestamp"`
}

// DailySpO2Item from GET /v2/usercollection/daily_spo2.
type DailySpO2Item struct {
	ID             string          `json:"id"`
	Day            string          `json:"day"`
	SpO2Percentage *SpO2Percentage `json:"spo2_percentage"`
}

// SpO2Percentage holds the average SpO2 recorded during sleep.
type SpO2Percentage struct {
	Average float64 `json:"average"`
}

// DailyStressItem from GET /v2/usercollection/daily_stress.
type DailyStressItem struct {
	ID           string `json:"id"`
	Day          string `json:"day"`
	StressHigh   *int   `json:"stress_high"`
	RecoveryHigh *int   `json:"recovery_high"`
}

// DailyResilienceItem from GET /v2/usercollection/daily_resilience.
type DailyResilienceItem struct {
	ID    string `json:"id"`
	Day   string `json:"day"`
	Level string `json:"level"`
}

// DailyCardiovascularAgeItem from GET /v2/usercollection/daily_cardiovascular_age.
type DailyCardiovascularAgeItem struct {
	Day         string `json:"day"`
	VascularAge *int   `json:"vascular_age"`
}

// VO2MaxItem from GET /v2/usercollection/vo2_max.
type VO2MaxItem struct {
	ID        string   `json:"id"`
	Day       string   `json:"day"`
	Timestamp string   `json:"timestamp"`
	VO2Max    *float64 `json:"vo2_max"`
}

// WorkoutItem from GET /v2/usercollection/workout.
type WorkoutItem struct {
	ID            string   `json:"id"`
	Activity      string   `json:"activity"`
	Calories      *float64 `json:"calories"`
	Day           string   `json:"day"`
	Distance      *float64 `json:"distance"`
	EndDatetime   string   `json:"end_datetime"`
	Intensity     string   `json:"intensity"`
	Label         *string  `json:"label"`
	Source        string   `json:"source"`
	StartDatetime string   `json:"start_datetime"`
}
