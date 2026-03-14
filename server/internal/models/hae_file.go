package models

import "time"

// AppleEpochOffset is the number of seconds between Unix epoch (1970-01-01)
// and Apple Core Data epoch (2001-01-01).
const AppleEpochOffset int64 = 978307200

// AppleTimestampToTime converts an Apple Core Data timestamp (seconds since 2001-01-01)
// to a Go time.Time in UTC.
func AppleTimestampToTime(appleTS float64) time.Time {
	sec := int64(appleTS)
	nsec := int64((appleTS - float64(sec)) * 1e9)
	return time.Unix(sec+AppleEpochOffset, nsec).UTC()
}

// HAEFileMetric is the root JSON structure of a health metric .hae file.
type HAEFileMetric struct {
	Metric string             `json:"metric"`
	Date   float64            `json:"date"`
	Data   []HAEFileDataPoint `json:"data"`
}

// HAEFileDataPoint is a single data point within a health metric .hae file.
// Standard metrics use Qty; heart_rate uses Min/Avg/Max (lowercase in .hae files).
// Sleep uses Awake/Core/Deep/REM plus TotalSleep.
type HAEFileDataPoint struct {
	Metric string           `json:"metric"`
	Start  float64          `json:"start"`
	End    float64          `json:"end"`
	Unit   string           `json:"unit"`
	Qty    *float64         `json:"qty,omitempty"`
	Min    *float64         `json:"min,omitempty"`
	Avg    *float64         `json:"avg,omitempty"`
	Max    *float64         `json:"max,omitempty"`
	Sources []HAEFileSource `json:"sources,omitempty"`

	// Sleep-specific fields
	TotalSleep *float64 `json:"totalSleep,omitempty"`
	Awake      *float64 `json:"awake,omitempty"`
	Core       *float64 `json:"core,omitempty"`
	Deep       *float64 `json:"deep,omitempty"`
	REM        *float64 `json:"rem,omitempty"`
}

// HAEFileSource identifies the data source device.
type HAEFileSource struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

// SleepStageType returns the sleep stage name for this data point,
// or empty string if no sleep stage field is present.
func (dp *HAEFileDataPoint) SleepStageType() string {
	if dp.Awake != nil && *dp.Awake > 0 {
		return "Awake"
	}
	if dp.Core != nil && *dp.Core > 0 {
		return "Core"
	}
	if dp.Deep != nil && *dp.Deep > 0 {
		return "Deep"
	}
	if dp.REM != nil && *dp.REM > 0 {
		return "REM"
	}
	return ""
}

// SleepStageDuration returns the duration in hours for the detected sleep stage.
func (dp *HAEFileDataPoint) SleepStageDuration() float64 {
	if dp.Awake != nil && *dp.Awake > 0 {
		return *dp.Awake
	}
	if dp.Core != nil && *dp.Core > 0 {
		return *dp.Core
	}
	if dp.Deep != nil && *dp.Deep > 0 {
		return *dp.Deep
	}
	if dp.REM != nil && *dp.REM > 0 {
		return *dp.REM
	}
	return 0
}

// SourceName returns the first source's name, or empty string.
func (dp *HAEFileDataPoint) SourceName() string {
	if len(dp.Sources) > 0 {
		return dp.Sources[0].Name
	}
	return ""
}

// HAEFileWorkout is the JSON structure of a workout .hae file.
type HAEFileWorkout struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Start         float64  `json:"start"`
	End           float64  `json:"end"`
	Duration      float64  `json:"duration"`
	ActiveEnergy  *float64 `json:"activeEnergy,omitempty"`
	TotalDistance  *float64 `json:"totalDistance,omitempty"`
	ElevationUp   *float64 `json:"elevationUp,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	Humidity      *float64 `json:"humidity,omitempty"`
	METs          *float64 `json:"METs,omitempty"`
	Location      string   `json:"location,omitempty"`
}

// HAEFileRoute is the JSON structure of a route .hae file.
type HAEFileRoute struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Locations []HAEFileLocation `json:"locations"`
}

// HAEFileLocation is a single GPS point in a route.
type HAEFileLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Elevation float64 `json:"elevation"`
	Speed     float64 `json:"speed"`
	Course    float64 `json:"course"`
	Time      float64 `json:"time"`
	HAcc      float64 `json:"hAcc"`
	VAcc      float64 `json:"vAcc"`
}
