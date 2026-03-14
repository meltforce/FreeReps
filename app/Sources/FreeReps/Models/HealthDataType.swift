import Foundation
import HealthKit

// MARK: - Category groupings

enum HealthCategory: String, CaseIterable, Identifiable {
    case activity = "Activity"
    case body = "Body Measurements"
    case vitals = "Vitals"
    case mobility = "Mobility & Fitness"
    case lab = "Lab & Clinical"
    case hearing = "Hearing"
    case respiratory = "Respiratory"
    case nutrition = "Nutrition"
    case sleep = "Sleep"
    case mindfulness = "Mindfulness"
    case reproductive = "Reproductive Health"
    case heartEvents = "Heart Events"
    case other = "Other"
    case workouts = "Workouts"
    case bloodPressure = "Blood Pressure"
    case ecg = "ECG"
    case audiogram = "Audiogram"
    case medications = "Medications"
    case symptoms = "Symptoms"

    var id: String { rawValue }

    var systemImage: String {
        switch self {
        case .activity: return "figure.walk"
        case .body: return "person.fill"
        case .vitals: return "heart.fill"
        case .mobility: return "figure.run"
        case .lab: return "cross.vial.fill"
        case .hearing: return "ear.fill"
        case .respiratory: return "lungs.fill"
        case .nutrition: return "fork.knife"
        case .sleep: return "bed.double.fill"
        case .mindfulness: return "brain.head.profile"
        case .reproductive: return "figure.2.and.child.holdinghands"
        case .heartEvents: return "waveform.path.ecg"
        case .other: return "list.bullet"
        case .workouts: return "dumbbell.fill"
        case .bloodPressure: return "drop.fill"
        case .ecg: return "waveform.path.ecg.rectangle.fill"
        case .audiogram: return "ear.badge.waveform"
        case .medications: return "pills.fill"
        case .symptoms: return "medical.thermometer"
        }
    }
}

// MARK: - Quantity type descriptor

/// How a quantity type should be synced.
enum QuantitySyncStrategy {
    /// Send each HKQuantitySample individually (low-frequency types like weight, resting HR).
    case individual
    /// Aggregate on-device into time buckets with min/avg/max (high-frequency types like heart rate).
    case aggregate(interval: TimeInterval)
    /// Aggregate cumulative metrics into time buckets with SUM (steps, energy, distance, etc.).
    case aggregateCumulative(interval: TimeInterval)
}

struct QuantityTypeDescriptor: Identifiable {
    let id: String          // HKQuantityTypeIdentifier raw value
    let displayName: String
    let category: HealthCategory
    let unit: HKUnit
    let unitString: String  // for DB storage
    var syncStrategy: QuantitySyncStrategy = .individual

    var hkIdentifier: HKQuantityTypeIdentifier { HKQuantityTypeIdentifier(rawValue: id) }
    var hkType: HKQuantityType? { HKObjectType.quantityType(forIdentifier: hkIdentifier) }
}

// MARK: - Category type descriptor

struct CategoryTypeDescriptor: Identifiable {
    let id: String          // HKCategoryTypeIdentifier raw value
    let displayName: String
    let category: HealthCategory
    let valueLabels: [Int: String]

    var hkIdentifier: HKCategoryTypeIdentifier { HKCategoryTypeIdentifier(rawValue: id) }
    var hkType: HKCategoryType? { HKObjectType.categoryType(forIdentifier: hkIdentifier) }
}

// MARK: - Master registry

enum HealthDataTypes {

    // MARK: Quantity types

    static let allQuantityTypes: [QuantityTypeDescriptor] = [
        // Activity
        .init(id: HKQuantityTypeIdentifier.stepCount.rawValue, displayName: "Steps", category: .activity, unit: .count(), unitString: "count", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.distanceWalkingRunning.rawValue, displayName: "Walking+Running Distance", category: .activity, unit: .meter(), unitString: "m", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.distanceCycling.rawValue, displayName: "Cycling Distance", category: .activity, unit: .meter(), unitString: "m", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.distanceSwimming.rawValue, displayName: "Swimming Distance", category: .activity, unit: .meter(), unitString: "m", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.distanceWheelchair.rawValue, displayName: "Wheelchair Distance", category: .activity, unit: .meter(), unitString: "m", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.basalEnergyBurned.rawValue, displayName: "Resting Energy", category: .activity, unit: .kilocalorie(), unitString: "kcal", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.activeEnergyBurned.rawValue, displayName: "Active Energy", category: .activity, unit: .kilocalorie(), unitString: "kcal", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.flightsClimbed.rawValue, displayName: "Flights Climbed", category: .activity, unit: .count(), unitString: "count", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.appleExerciseTime.rawValue, displayName: "Exercise Minutes", category: .activity, unit: .minute(), unitString: "min", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.appleMoveTime.rawValue, displayName: "Move Time", category: .activity, unit: .minute(), unitString: "min", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.appleStandTime.rawValue, displayName: "Stand Time", category: .activity, unit: .minute(), unitString: "min", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.pushCount.rawValue, displayName: "Wheelchair Pushes", category: .activity, unit: .count(), unitString: "count", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.swimmingStrokeCount.rawValue, displayName: "Swimming Strokes", category: .activity, unit: .count(), unitString: "count", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.distanceDownhillSnowSports.rawValue, displayName: "Downhill Snow Sports Distance", category: .activity, unit: .meter(), unitString: "m", syncStrategy: .aggregateCumulative(interval: 3600)),
        // Body
        .init(id: HKQuantityTypeIdentifier.bodyMass.rawValue, displayName: "Weight", category: .body, unit: .gramUnit(with: .kilo), unitString: "kg"),
        .init(id: HKQuantityTypeIdentifier.bodyMassIndex.rawValue, displayName: "BMI", category: .body, unit: .count(), unitString: "kg/m²"),
        .init(id: HKQuantityTypeIdentifier.bodyFatPercentage.rawValue, displayName: "Body Fat %", category: .body, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.height.rawValue, displayName: "Height", category: .body, unit: .meter(), unitString: "m"),
        .init(id: HKQuantityTypeIdentifier.leanBodyMass.rawValue, displayName: "Lean Body Mass", category: .body, unit: .gramUnit(with: .kilo), unitString: "kg"),
        .init(id: HKQuantityTypeIdentifier.waistCircumference.rawValue, displayName: "Waist Circumference", category: .body, unit: .meter(), unitString: "m"),
        // Vitals
        .init(id: HKQuantityTypeIdentifier.heartRate.rawValue, displayName: "Heart Rate", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "bpm"),
        .init(id: HKQuantityTypeIdentifier.restingHeartRate.rawValue, displayName: "Resting Heart Rate", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "bpm"),
        .init(id: HKQuantityTypeIdentifier.walkingHeartRateAverage.rawValue, displayName: "Walking Heart Rate", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "bpm"),
        .init(id: HKQuantityTypeIdentifier.heartRateVariabilitySDNN.rawValue, displayName: "HRV (SDNN)", category: .vitals, unit: .secondUnit(with: .milli), unitString: "ms"),
        .init(id: HKQuantityTypeIdentifier.oxygenSaturation.rawValue, displayName: "Blood Oxygen", category: .vitals, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.bodyTemperature.rawValue, displayName: "Body Temperature", category: .vitals, unit: .degreeCelsius(), unitString: "°C"),
        .init(id: HKQuantityTypeIdentifier.basalBodyTemperature.rawValue, displayName: "Basal Body Temperature", category: .vitals, unit: .degreeCelsius(), unitString: "°C"),
        .init(id: HKQuantityTypeIdentifier.respiratoryRate.rawValue, displayName: "Respiratory Rate", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "breaths/min"),
        .init(id: HKQuantityTypeIdentifier.bloodPressureSystolic.rawValue, displayName: "Systolic BP", category: .bloodPressure, unit: .millimeterOfMercury(), unitString: "mmHg"),
        .init(id: HKQuantityTypeIdentifier.bloodPressureDiastolic.rawValue, displayName: "Diastolic BP", category: .bloodPressure, unit: .millimeterOfMercury(), unitString: "mmHg"),
        .init(id: HKQuantityTypeIdentifier.peripheralPerfusionIndex.rawValue, displayName: "Peripheral Perfusion Index", category: .vitals, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.electrodermalActivity.rawValue, displayName: "Electrodermal Activity", category: .vitals, unit: HKUnit(from: "S"), unitString: "S"),
        // Mobility & Fitness
        .init(id: HKQuantityTypeIdentifier.vo2Max.rawValue, displayName: "VO2 Max", category: .mobility, unit: HKUnit.literUnit(with: .milli).unitDivided(by: HKUnit.gramUnit(with: .kilo).unitMultiplied(by: HKUnit.minute())), unitString: "mL/kg·min"),
        .init(id: HKQuantityTypeIdentifier.walkingSpeed.rawValue, displayName: "Walking Speed", category: .mobility, unit: HKUnit(from: "m/s"), unitString: "m/s"),
        .init(id: HKQuantityTypeIdentifier.walkingStepLength.rawValue, displayName: "Walking Step Length", category: .mobility, unit: .meter(), unitString: "m"),
        .init(id: HKQuantityTypeIdentifier.walkingAsymmetryPercentage.rawValue, displayName: "Walking Asymmetry", category: .mobility, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.walkingDoubleSupportPercentage.rawValue, displayName: "Double Support Time", category: .mobility, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.stairAscentSpeed.rawValue, displayName: "Stair Ascent Speed", category: .mobility, unit: HKUnit(from: "m/s"), unitString: "m/s"),
        .init(id: HKQuantityTypeIdentifier.stairDescentSpeed.rawValue, displayName: "Stair Descent Speed", category: .mobility, unit: HKUnit(from: "m/s"), unitString: "m/s"),
        .init(id: HKQuantityTypeIdentifier.sixMinuteWalkTestDistance.rawValue, displayName: "6-Min Walk Distance", category: .mobility, unit: .meter(), unitString: "m"),
        .init(id: HKQuantityTypeIdentifier.runningStrideLength.rawValue, displayName: "Running Stride Length", category: .mobility, unit: .meter(), unitString: "m"),
        .init(id: HKQuantityTypeIdentifier.runningVerticalOscillation.rawValue, displayName: "Running Vertical Oscillation", category: .mobility, unit: .meter(), unitString: "m"),
        .init(id: HKQuantityTypeIdentifier.runningGroundContactTime.rawValue, displayName: "Ground Contact Time", category: .mobility, unit: .secondUnit(with: .milli), unitString: "ms"),
        .init(id: HKQuantityTypeIdentifier.runningPower.rawValue, displayName: "Running Power", category: .mobility, unit: HKUnit(from: "W"), unitString: "W"),
        .init(id: HKQuantityTypeIdentifier.runningSpeed.rawValue, displayName: "Running Speed", category: .mobility, unit: HKUnit(from: "m/s"), unitString: "m/s"),
        // Lab & Clinical
        .init(id: HKQuantityTypeIdentifier.bloodGlucose.rawValue, displayName: "Blood Glucose", category: .lab, unit: HKUnit(from: "mg/dL"), unitString: "mg/dL"),
        .init(id: HKQuantityTypeIdentifier.insulinDelivery.rawValue, displayName: "Insulin Delivery", category: .lab, unit: HKUnit(from: "IU"), unitString: "IU"),
        .init(id: HKQuantityTypeIdentifier.bloodAlcoholContent.rawValue, displayName: "Blood Alcohol Content", category: .lab, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.numberOfTimesFallen.rawValue, displayName: "Falls", category: .lab, unit: .count(), unitString: "count"),
        // Hearing
        .init(id: HKQuantityTypeIdentifier.environmentalAudioExposure.rawValue, displayName: "Environmental Audio Exposure", category: .hearing, unit: HKUnit.decibelAWeightedSoundPressureLevel(), unitString: "dBASPL"),
        .init(id: HKQuantityTypeIdentifier.headphoneAudioExposure.rawValue, displayName: "Headphone Audio Exposure", category: .hearing, unit: HKUnit.decibelAWeightedSoundPressureLevel(), unitString: "dBASPL"),
        // Respiratory
        .init(id: HKQuantityTypeIdentifier.forcedVitalCapacity.rawValue, displayName: "Forced Vital Capacity", category: .respiratory, unit: .liter(), unitString: "L"),
        .init(id: HKQuantityTypeIdentifier.forcedExpiratoryVolume1.rawValue, displayName: "FEV1", category: .respiratory, unit: .liter(), unitString: "L"),
        .init(id: HKQuantityTypeIdentifier.peakExpiratoryFlowRate.rawValue, displayName: "Peak Expiratory Flow", category: .respiratory, unit: HKUnit(from: "L/min"), unitString: "L/min"),
        .init(id: HKQuantityTypeIdentifier.inhalerUsage.rawValue, displayName: "Inhaler Usage", category: .respiratory, unit: .count(), unitString: "count"),
        .init(id: HKQuantityTypeIdentifier.uvExposure.rawValue, displayName: "UV Exposure", category: .respiratory, unit: .count(), unitString: "count"),
        // Nutrition
        .init(id: HKQuantityTypeIdentifier.dietaryEnergyConsumed.rawValue, displayName: "Dietary Energy", category: .nutrition, unit: .kilocalorie(), unitString: "kcal"),
        .init(id: HKQuantityTypeIdentifier.dietaryProtein.rawValue, displayName: "Protein", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryCarbohydrates.rawValue, displayName: "Carbohydrates", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryFatTotal.rawValue, displayName: "Total Fat", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryFatSaturated.rawValue, displayName: "Saturated Fat", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryFatMonounsaturated.rawValue, displayName: "Monounsaturated Fat", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryFatPolyunsaturated.rawValue, displayName: "Polyunsaturated Fat", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietarySugar.rawValue, displayName: "Sugar", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryFiber.rawValue, displayName: "Fiber", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryCholesterol.rawValue, displayName: "Cholesterol", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietarySodium.rawValue, displayName: "Sodium", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryCalcium.rawValue, displayName: "Calcium", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryPhosphorus.rawValue, displayName: "Phosphorus", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryMagnesium.rawValue, displayName: "Magnesium", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryPotassium.rawValue, displayName: "Potassium", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryIron.rawValue, displayName: "Iron", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryZinc.rawValue, displayName: "Zinc", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryManganese.rawValue, displayName: "Manganese", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryCopper.rawValue, displayName: "Copper", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietarySelenium.rawValue, displayName: "Selenium", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryChromium.rawValue, displayName: "Chromium", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryMolybdenum.rawValue, displayName: "Molybdenum", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryIodine.rawValue, displayName: "Iodine", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryVitaminA.rawValue, displayName: "Vitamin A", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryVitaminB6.rawValue, displayName: "Vitamin B6", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryVitaminB12.rawValue, displayName: "Vitamin B12", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryVitaminC.rawValue, displayName: "Vitamin C", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryVitaminD.rawValue, displayName: "Vitamin D", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryVitaminE.rawValue, displayName: "Vitamin E", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryVitaminK.rawValue, displayName: "Vitamin K", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryThiamin.rawValue, displayName: "Thiamin", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryRiboflavin.rawValue, displayName: "Riboflavin", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryNiacin.rawValue, displayName: "Niacin", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryPantothenicAcid.rawValue, displayName: "Pantothenic Acid", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryFolate.rawValue, displayName: "Folate", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryBiotin.rawValue, displayName: "Biotin", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryCaffeine.rawValue, displayName: "Caffeine", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryWater.rawValue, displayName: "Water", category: .nutrition, unit: .liter(), unitString: "L"),
        .init(id: HKQuantityTypeIdentifier.dietaryChloride.rawValue, displayName: "Chloride", category: .nutrition, unit: .gram(), unitString: "g"),
        // iOS 15+
        .init(id: HKQuantityTypeIdentifier.appleWalkingSteadiness.rawValue, displayName: "Walking Steadiness", category: .mobility, unit: .percent(), unitString: "%"),
        // iOS 16+
        .init(id: HKQuantityTypeIdentifier.heartRateRecoveryOneMinute.rawValue, displayName: "Heart Rate Recovery", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "bpm"),
        .init(id: HKQuantityTypeIdentifier.atrialFibrillationBurden.rawValue, displayName: "AFib Burden", category: .vitals, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.appleSleepingWristTemperature.rawValue, displayName: "Sleeping Wrist Temperature", category: .body, unit: .degreeCelsius(), unitString: "°C"),
        .init(id: HKQuantityTypeIdentifier.underwaterDepth.rawValue, displayName: "Underwater Depth", category: .other, unit: .meter(), unitString: "m"),
        .init(id: HKQuantityTypeIdentifier.waterTemperature.rawValue, displayName: "Water Temperature", category: .other, unit: .degreeCelsius(), unitString: "°C"),
        // iOS 17+ — raw string IDs because the deployment target is iOS 16
        .init(id: "HKQuantityTypeIdentifierTimeInDaylight", displayName: "Time in Daylight", category: .activity, unit: .second(), unitString: "s"),
        .init(id: "HKQuantityTypeIdentifierPhysicalEffort", displayName: "Physical Effort", category: .activity,
              unit: HKUnit.kilocalorie().unitDivided(by: HKUnit.gramUnit(with: .kilo).unitMultiplied(by: HKUnit.hour())),
              unitString: "MET"),
        .init(id: "HKQuantityTypeIdentifierCyclingSpeed", displayName: "Cycling Speed", category: .mobility, unit: HKUnit(from: "m/s"), unitString: "m/s"),
        .init(id: "HKQuantityTypeIdentifierCyclingPower", displayName: "Cycling Power", category: .mobility, unit: HKUnit(from: "W"), unitString: "W"),
        .init(id: "HKQuantityTypeIdentifierCyclingFunctionalThresholdPower", displayName: "Cycling Threshold Power", category: .mobility, unit: HKUnit(from: "W"), unitString: "W"),
        .init(id: "HKQuantityTypeIdentifierCyclingCadence", displayName: "Cycling Cadence", category: .mobility, unit: HKUnit(from: "count/min"), unitString: "rpm"),
        // iOS 18+ — raw string IDs
        .init(id: "HKQuantityTypeIdentifierEstimatedWorkoutEffortScore", displayName: "Estimated Workout Effort", category: .activity, unit: HKUnit(from: "appleEffortScore"), unitString: "score"),
        .init(id: "HKQuantityTypeIdentifierWorkoutEffortScore", displayName: "Workout Effort Score", category: .activity, unit: HKUnit(from: "appleEffortScore"), unitString: "score"),
    ]

    // MARK: Category types

    static let allCategoryTypes: [CategoryTypeDescriptor] = [
        .init(id: HKCategoryTypeIdentifier.sleepAnalysis.rawValue, displayName: "Sleep Analysis", category: .sleep, valueLabels: [
            0: "In Bed", 1: "Asleep Unspecified", 2: "Awake", 3: "Asleep Core", 4: "Asleep Deep", 5: "Asleep REM"
        ]),
        .init(id: HKCategoryTypeIdentifier.appleStandHour.rawValue, displayName: "Stand Hour", category: .activity, valueLabels: [0: "Idle", 1: "Stood"]),
        .init(id: HKCategoryTypeIdentifier.cervicalMucusQuality.rawValue, displayName: "Cervical Mucus Quality", category: .reproductive, valueLabels: [1: "Dry", 2: "Sticky", 3: "Creamy", 4: "Watery", 5: "Egg White"]),
        .init(id: HKCategoryTypeIdentifier.menstrualFlow.rawValue, displayName: "Menstrual Flow", category: .reproductive, valueLabels: [1: "Unspecified", 2: "Light", 3: "Medium", 4: "Heavy", 5: "None"]),
        .init(id: HKCategoryTypeIdentifier.intermenstrualBleeding.rawValue, displayName: "Intermenstrual Bleeding", category: .reproductive, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.ovulationTestResult.rawValue, displayName: "Ovulation Test Result", category: .reproductive, valueLabels: [1: "Negative", 2: "LH Surge", 3: "Indeterminate", 4: "Positive"]),
        .init(id: HKCategoryTypeIdentifier.sexualActivity.rawValue, displayName: "Sexual Activity", category: .reproductive, valueLabels: [0: "Unspecified"]),
        .init(id: HKCategoryTypeIdentifier.mindfulSession.rawValue, displayName: "Mindful Session", category: .mindfulness, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.highHeartRateEvent.rawValue, displayName: "High Heart Rate Event", category: .heartEvents, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.lowHeartRateEvent.rawValue, displayName: "Low Heart Rate Event", category: .heartEvents, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.irregularHeartRhythmEvent.rawValue, displayName: "Irregular Heart Rhythm", category: .heartEvents, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.environmentalAudioExposureEvent.rawValue, displayName: "Environmental Audio Event", category: .hearing, valueLabels: [0: "Momentary Limit"]),
        .init(id: HKCategoryTypeIdentifier.headphoneAudioExposureEvent.rawValue, displayName: "Headphone Audio Event", category: .hearing, valueLabels: [0: "SevenDay Limit"]),
        .init(id: HKCategoryTypeIdentifier.toothbrushingEvent.rawValue, displayName: "Toothbrushing", category: .other, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.handwashingEvent.rawValue, displayName: "Handwashing", category: .other, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.lactation.rawValue, displayName: "Lactation", category: .reproductive, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.pregnancy.rawValue, displayName: "Pregnancy", category: .reproductive, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.pregnancyTestResult.rawValue, displayName: "Pregnancy Test", category: .reproductive, valueLabels: [1: "Negative", 2: "Positive", 3: "Indeterminate"]),
        .init(id: HKCategoryTypeIdentifier.progesteroneTestResult.rawValue, displayName: "Progesterone Test", category: .reproductive, valueLabels: [1: "Negative", 2: "Positive", 3: "Indeterminate"]),
        .init(id: HKCategoryTypeIdentifier.contraceptive.rawValue, displayName: "Contraceptive", category: .reproductive, valueLabels: [1: "Unspecified", 2: "Implant", 3: "Injection", 4: "IUD", 5: "Intravaginal", 6: "Oral", 7: "Patch"]),
        // iOS 14.3+
        .init(id: HKCategoryTypeIdentifier.lowCardioFitnessEvent.rawValue, displayName: "Low Cardio Fitness Event", category: .heartEvents, valueLabels: [0: "Low Fitness"]),
        // iOS 15+
        .init(id: HKCategoryTypeIdentifier.appleWalkingSteadinessEvent.rawValue, displayName: "Walking Steadiness Event", category: .mobility, valueLabels: [1: "Initial Low", 2: "Initial Very Low", 3: "Repeat Low", 4: "Repeat Very Low"]),
        // iOS 16+
        .init(id: HKCategoryTypeIdentifier.irregularMenstrualCycles.rawValue, displayName: "Irregular Menstrual Cycles", category: .reproductive, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.infrequentMenstrualCycles.rawValue, displayName: "Infrequent Menstrual Cycles", category: .reproductive, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.persistentIntermenstrualBleeding.rawValue, displayName: "Persistent Intermenstrual Bleeding", category: .reproductive, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.prolongedMenstrualPeriods.rawValue, displayName: "Prolonged Menstrual Periods", category: .reproductive, valueLabels: [0: "Present"]),
        // iOS 18+ — raw string IDs
        .init(id: "HKCategoryTypeIdentifierBleedingDuringPregnancy", displayName: "Bleeding During Pregnancy", category: .reproductive, valueLabels: [0: "Present"]),
        .init(id: "HKCategoryTypeIdentifierBleedingAfterPregnancy", displayName: "Bleeding After Pregnancy", category: .reproductive, valueLabels: [0: "Present"]),
        // iOS 14.0+ — Cardiovascular events
        .init(id: HKCategoryTypeIdentifier.skippedHeartbeat.rawValue,
              displayName: "Skipped Heartbeat", category: .heartEvents, valueLabels: [0: "Present"]),
        .init(id: HKCategoryTypeIdentifier.rapidPoundingOrFlutteringHeartbeat.rawValue,
              displayName: "Rapid/Pounding Heartbeat", category: .heartEvents, valueLabels: [0: "Present"]),
        // iOS 17.0+ — raw string ID
        .init(id: "HKCategoryTypeIdentifierHypertensionEvent",
              displayName: "Hypertension Event", category: .heartEvents, valueLabels: [0: "Present"]),
        // iOS 14.0+ — Symptoms (HKCategoryValuePresence: 0=Not Present, 1=Present)
        .init(id: HKCategoryTypeIdentifier.abdominalCramps.rawValue,
              displayName: "Abdominal Cramps", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.acne.rawValue,
              displayName: "Acne", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.appetiteChanges.rawValue,
              displayName: "Appetite Changes", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.generalizedBodyAche.rawValue,
              displayName: "Generalized Body Ache", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.bloating.rawValue,
              displayName: "Bloating", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.breastPain.rawValue,
              displayName: "Breast Pain", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.chestTightnessOrPain.rawValue,
              displayName: "Chest Tightness or Pain", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.chills.rawValue,
              displayName: "Chills", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.constipation.rawValue,
              displayName: "Constipation", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.coughing.rawValue,
              displayName: "Coughing", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.diarrhea.rawValue,
              displayName: "Diarrhea", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.dizziness.rawValue,
              displayName: "Dizziness", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.fainting.rawValue,
              displayName: "Fainting", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.fatigue.rawValue,
              displayName: "Fatigue", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.fever.rawValue,
              displayName: "Fever", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.headache.rawValue,
              displayName: "Headache", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.heartburn.rawValue,
              displayName: "Heartburn", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.hotFlashes.rawValue,
              displayName: "Hot Flashes", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.lowerBackPain.rawValue,
              displayName: "Lower Back Pain", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.lossOfSmell.rawValue,
              displayName: "Loss of Smell", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.lossOfTaste.rawValue,
              displayName: "Loss of Taste", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.moodChanges.rawValue,
              displayName: "Mood Changes", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.nausea.rawValue,
              displayName: "Nausea", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.pelvicPain.rawValue,
              displayName: "Pelvic Pain", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.runnyNose.rawValue,
              displayName: "Runny Nose", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.shortnessOfBreath.rawValue,
              displayName: "Shortness of Breath", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.sinusCongestion.rawValue,
              displayName: "Sinus Congestion", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.soreThroat.rawValue,
              displayName: "Sore Throat", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.vomiting.rawValue,
              displayName: "Vomiting", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.wheezing.rawValue,
              displayName: "Wheezing", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.bladderIncontinence.rawValue,
              displayName: "Bladder Incontinence", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.drySkin.rawValue,
              displayName: "Dry Skin", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.hairLoss.rawValue,
              displayName: "Hair Loss", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.vaginalDryness.rawValue,
              displayName: "Vaginal Dryness", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.memoryLapse.rawValue,
              displayName: "Memory Lapse", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
        .init(id: HKCategoryTypeIdentifier.nightSweats.rawValue,
              displayName: "Night Sweats", category: .symptoms, valueLabels: [0: "Not Present", 1: "Present"]),
    ]

    // MARK: All read types for HealthKit permissions

    static var allReadTypes: Set<HKObjectType> {
        var types = Set<HKObjectType>()
        for qt in allQuantityTypes {
            if let t = qt.hkType { types.insert(t) }
        }
        for ct in allCategoryTypes {
            if let t = ct.hkType { types.insert(t) }
        }
        types.insert(HKObjectType.workoutType())
        types.insert(HKObjectType.electrocardiogramType())
        types.insert(HKObjectType.audiogramSampleType())
        types.insert(HKSeriesType.workoutRoute())
        types.insert(HKObjectType.activitySummaryType())
        // HKVisionPrescriptionType uses requestPerObjectReadAuthorization — excluded here
        if #available(iOS 18, *) {
            types.insert(HKObjectType.stateOfMindType())
        }
        return types
    }

    // MARK: Lookup helpers

    static func quantityDescriptor(for id: String) -> QuantityTypeDescriptor? {
        allQuantityTypes.first { $0.id == id }
    }

    static func categoryDescriptor(for id: String) -> CategoryTypeDescriptor? {
        allCategoryTypes.first { $0.id == id }
    }

    // MARK: Display name for any HKObjectType

    static func displayName(for type: HKObjectType) -> String {
        if let qt = allQuantityTypes.first(where: { $0.hkType == type }) {
            return qt.displayName
        }
        if let ct = allCategoryTypes.first(where: { $0.hkType == type }) {
            return ct.displayName
        }
        if type == HKObjectType.workoutType() { return "Workouts" }
        if type == HKObjectType.electrocardiogramType() { return "ECG Recordings" }
        if type == HKObjectType.audiogramSampleType() { return "Audiograms" }
        if type == HKSeriesType.workoutRoute() { return "Workout Routes (GPS)" }
        if type == HKObjectType.activitySummaryType() { return "Activity Summaries" }
        return type.identifier
    }

    static func systemImage(for type: HKObjectType) -> String {
        if let qt = allQuantityTypes.first(where: { $0.hkType == type }) {
            return qt.category.systemImage
        }
        if let ct = allCategoryTypes.first(where: { $0.hkType == type }) {
            return ct.category.systemImage
        }
        if type == HKObjectType.workoutType() { return "dumbbell.fill" }
        if type == HKObjectType.electrocardiogramType() { return "waveform.path.ecg.rectangle.fill" }
        if type == HKObjectType.audiogramSampleType() { return "ear.badge.waveform" }
        if type == HKSeriesType.workoutRoute() { return "map.fill" }
        if type == HKObjectType.activitySummaryType() { return "chart.bar.fill" }
        return "heart.fill"
    }

    // MARK: Grouping

    static var quantityTypesByCategory: [(HealthCategory, [QuantityTypeDescriptor])] {
        var map: [HealthCategory: [QuantityTypeDescriptor]] = [:]
        for t in allQuantityTypes {
            map[t.category, default: []].append(t)
        }
        return HealthCategory.allCases.compactMap { cat in
            guard let types = map[cat], !types.isEmpty else { return nil }
            return (cat, types)
        }
    }
}
