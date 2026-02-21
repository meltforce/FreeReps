package models

import "strings"

// Canonical sleep stage names (as used by Apple Health in English).
const (
	SleepStageCore   = "Core"
	SleepStageDeep   = "Deep"
	SleepStageREM    = "REM"
	SleepStageAwake  = "Awake"
	SleepStageInBed  = "In Bed"
	SleepStageAsleep = "Asleep"
)

// sleepStageMap maps lowercased localized sleep stage names to their canonical
// English equivalents. Covers: English, German, French, Spanish, Italian,
// Portuguese, Dutch, Japanese, Chinese (Simplified & Traditional), Korean.
var sleepStageMap = map[string]string{
	// English
	"core":   SleepStageCore,
	"deep":   SleepStageDeep,
	"rem":    SleepStageREM,
	"awake":  SleepStageAwake,
	"in bed": SleepStageInBed,
	"asleep": SleepStageAsleep,

	// German
	"kern":    SleepStageCore,
	"tief":    SleepStageDeep,
	"wach":    SleepStageAwake,
	"im bett": SleepStageInBed,

	// French
	"paradoxal":  SleepStageREM,
	"profond":    SleepStageDeep,
	"léger":      SleepStageCore,
	"leger":      SleepStageCore,
	"éveillé":    SleepStageAwake,
	"eveille":    SleepStageAwake,
	"au lit":     SleepStageInBed,
	"endormi":    SleepStageAsleep,

	// Spanish (principal also covers Portuguese)
	"profundo":   SleepStageDeep,
	"principal":  SleepStageCore,
	"despierto":  SleepStageAwake,
	"despierta":  SleepStageAwake,
	"en la cama": SleepStageInBed,
	"dormido":    SleepStageAsleep,
	"dormida":    SleepStageAsleep,

	// Italian
	"profondo":      SleepStageDeep,
	"essenziale":    SleepStageCore,
	"sveglio":       SleepStageAwake,
	"sveglia":       SleepStageAwake,
	"a letto":       SleepStageInBed,
	"addormentato":  SleepStageAsleep,

	// Portuguese (principal already covered by Spanish, kern by German)
	"sono profundo": SleepStageDeep,
	"acordado":      SleepStageAwake,
	"acordada":      SleepStageAwake,
	"na cama":       SleepStageInBed,
	"dormindo":      SleepStageAsleep,

	// Dutch (kern already covered by German, in bed by English)
	"diep":    SleepStageDeep,
	"wakker":  SleepStageAwake,
	"slapend": SleepStageAsleep,

	// Japanese
	"コア":   SleepStageCore,
	"深い":   SleepStageDeep,
	"レム":   SleepStageREM,
	"覚醒":   SleepStageAwake,
	"ベッドで": SleepStageInBed,

	// Chinese (Simplified)
	"核心":  SleepStageCore,
	"深度":  SleepStageDeep,
	"快速眼动": SleepStageREM,
	"清醒":  SleepStageAwake,
	"在床上": SleepStageInBed,

	// Chinese (Traditional)
	"核心睡眠": SleepStageCore,
	"深層":    SleepStageDeep,
	"快速動眼": SleepStageREM,

	// Korean
	"코어":  SleepStageCore,
	"깊은":  SleepStageDeep,
	"렘":   SleepStageREM,
	"깨어있음": SleepStageAwake,
	"침대에서": SleepStageInBed,
}

// NormalizeSleepStage maps a possibly-localized sleep stage name to its
// canonical English equivalent. Returns the canonical name and true if
// recognized, or the original string and false if unknown.
func NormalizeSleepStage(raw string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if canonical, ok := sleepStageMap[lower]; ok {
		return canonical, true
	}
	return raw, false
}
