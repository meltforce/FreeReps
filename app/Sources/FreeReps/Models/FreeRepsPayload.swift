import Foundation
import HealthKit

// MARK: - Top-level payload matching HAEPayload

struct FreeRepsPayload: Encodable {
    var data: FreeRepsData
}

struct FreeRepsData: Encodable {
    var metrics: [FreeRepsMetric] = []
    var workouts: [FreeRepsWorkout] = []
    var ecg_recordings: [FreeRepsECG] = []
    var audiograms: [FreeRepsAudiogram] = []
    var activity_summaries: [FreeRepsActivitySummary] = []
    var medications: [FreeRepsMedication] = []
    var vision_prescriptions: [FreeRepsVisionPrescription] = []
    var state_of_mind: [FreeRepsStateOfMind] = []
    var category_samples: [FreeRepsCategorySample] = []
}

// MARK: - Metrics (quantity samples)

struct FreeRepsMetric: Encodable {
    let name: String
    let units: String
    let data: [FreeRepsMetricDataPoint]
}

struct FreeRepsMetricDataPoint: Encodable {
    let date: String
    var qty: Double = 0
    var source_uuid: String? = nil
    // Min/Avg/Max for aggregated types (heart rate). Maps to HAE ShapeMinAvgMax.
    var Min: Double? = nil
    var Avg: Double? = nil
    var Max: Double? = nil
}

// MARK: - Workouts

struct FreeRepsWorkout: Encodable {
    let id: String
    let name: String
    let start: String
    let end: String
    let duration: Double

    var location: String?
    var isIndoor: Bool?

    var activeEnergyBurned: FreeRepsQuantity?
    var totalEnergy: FreeRepsQuantity?
    var distance: FreeRepsQuantity?

    var elevationUp: FreeRepsQuantity?
    var elevationDown: FreeRepsQuantity?

    var heartRate: FreeRepsHRSummary?

    var heartRateData: [FreeRepsWorkoutHRPoint]?
    var route: [FreeRepsRoutePoint]?
}

struct FreeRepsQuantity: Encodable {
    let qty: Double
    let units: String
}

struct FreeRepsHRSummary: Encodable {
    let min: FreeRepsQuantity
    let avg: FreeRepsQuantity
    let max: FreeRepsQuantity
}

struct FreeRepsWorkoutHRPoint: Encodable {
    let date: String
    let Min: Double
    let Avg: Double
    let Max: Double
    let units: String
    let source: String
}

struct FreeRepsRoutePoint: Encodable {
    let latitude: Double
    let longitude: Double
    let altitude: Double
    let course: Double
    let courseAccuracy: Double
    let horizontalAccuracy: Double
    let verticalAccuracy: Double
    let timestamp: String
    let speed: Double
    let speedAccuracy: Double
}

// MARK: - ECG recordings

struct FreeRepsECG: Encodable {
    let id: String
    let classification: String
    let average_heart_rate: Double?
    let sampling_frequency: Double?
    let voltage_measurements: [Double]?
    let start_date: String
    let source: String
}

// MARK: - Audiograms

struct FreeRepsAudiogram: Encodable {
    let id: String
    let sensitivity_points: [AudiogramPoint]
    let start_date: String
    let source: String
}

struct AudiogramPoint: Encodable {
    let hz: Double
    let left_db: Double?
    let right_db: Double?
}

// MARK: - Activity summaries

struct FreeRepsActivitySummary: Encodable {
    let date: String
    let active_energy: Double?
    let active_energy_goal: Double?
    let exercise_time: Double?
    let exercise_time_goal: Double?
    let stand_hours: Double?
    let stand_hours_goal: Double?
}

// MARK: - Medications

struct FreeRepsMedication: Encodable {
    let id: String
    let name: String
    let dosage: String?
    let log_status: String?
    let start_date: String
    let end_date: String?
    let source: String
}

// MARK: - Vision prescriptions

struct FreeRepsVisionPrescription: Encodable {
    let id: String
    let date_issued: String
    let expiration_date: String?
    let prescription_type: String?
    let right_eye: [String: Double]?
    let left_eye: [String: Double]?
    let source: String
}

// MARK: - State of mind

struct FreeRepsStateOfMind: Encodable {
    let id: String
    let kind: Int
    let valence: Double
    let labels: [Int]
    let associations: [Int]
    let start_date: String
    let source: String
}

// MARK: - Category samples

struct FreeRepsCategorySample: Encodable {
    let id: String
    let type: String
    let value: Int
    let value_label: String?
    let start_date: String
    let end_date: String
    let source: String
}

// MARK: - Blood pressure (sent as metrics with systolic/diastolic shape)

struct FreeRepsBloodPressurePoint: Encodable {
    let date: String
    let systolic: Double
    let diastolic: Double
    let source_uuid: String?
}

// MARK: - Sleep stages (sent as category_samples)

// Sleep stages are sent as category_samples with type "sleep_analysis".
// FreeReps' existing sleep processing handles the unaggregated format:
// startDate/endDate/stage/qty → BackfillSleepSessions() groups into nightly summaries.

// MARK: - Date formatting

private let haeFormatter: DateFormatter = {
    let f = DateFormatter()
    f.dateFormat = "yyyy-MM-dd HH:mm:ss Z"
    f.locale = Locale(identifier: "en_US_POSIX")
    return f
}()

private let dateOnlyFormatter: DateFormatter = {
    let f = DateFormatter()
    f.dateFormat = "yyyy-MM-dd"
    f.locale = Locale(identifier: "en_US_POSIX")
    return f
}()

func haeDate(_ date: Date) -> String {
    haeFormatter.string(from: date)
}

func haeDateOnly(_ date: Date) -> String {
    dateOnlyFormatter.string(from: date)
}

// MARK: - Metric name mapping (HKQuantityTypeIdentifier → FreeReps snake_case)

/// Maps HKQuantityTypeIdentifier raw values to FreeReps snake_case metric names.
/// Must align with the metric_allowlist entries from migration 000008.
let hkToFreeRepsMetricName: [String: String] = [
    // Activity
    HKQuantityTypeIdentifier.stepCount.rawValue: "step_count",
    HKQuantityTypeIdentifier.distanceWalkingRunning.rawValue: "distance_walking_running",
    HKQuantityTypeIdentifier.distanceCycling.rawValue: "distance_cycling",
    HKQuantityTypeIdentifier.distanceSwimming.rawValue: "distance_swimming",
    HKQuantityTypeIdentifier.distanceWheelchair.rawValue: "distance_wheelchair",
    HKQuantityTypeIdentifier.basalEnergyBurned.rawValue: "basal_energy_burned",
    HKQuantityTypeIdentifier.activeEnergyBurned.rawValue: "active_energy",
    HKQuantityTypeIdentifier.flightsClimbed.rawValue: "flights_climbed",
    HKQuantityTypeIdentifier.appleExerciseTime.rawValue: "apple_exercise_time",
    HKQuantityTypeIdentifier.appleMoveTime.rawValue: "apple_move_time",
    HKQuantityTypeIdentifier.appleStandTime.rawValue: "apple_stand_time",
    HKQuantityTypeIdentifier.pushCount.rawValue: "push_count",
    HKQuantityTypeIdentifier.swimmingStrokeCount.rawValue: "swimming_stroke_count",
    HKQuantityTypeIdentifier.distanceDownhillSnowSports.rawValue: "distance_downhill_snow_sports",
    "HKQuantityTypeIdentifierTimeInDaylight": "time_in_daylight",
    "HKQuantityTypeIdentifierPhysicalEffort": "physical_effort",
    "HKQuantityTypeIdentifierEstimatedWorkoutEffortScore": "estimated_workout_effort_score",
    "HKQuantityTypeIdentifierWorkoutEffortScore": "workout_effort_score",
    // Body
    HKQuantityTypeIdentifier.bodyMass.rawValue: "weight_body_mass",
    HKQuantityTypeIdentifier.bodyMassIndex.rawValue: "body_mass_index",
    HKQuantityTypeIdentifier.bodyFatPercentage.rawValue: "body_fat_percentage",
    HKQuantityTypeIdentifier.height.rawValue: "height",
    HKQuantityTypeIdentifier.leanBodyMass.rawValue: "lean_body_mass",
    HKQuantityTypeIdentifier.waistCircumference.rawValue: "waist_circumference",
    HKQuantityTypeIdentifier.appleSleepingWristTemperature.rawValue: "apple_sleeping_wrist_temperature",
    // Vitals
    HKQuantityTypeIdentifier.heartRate.rawValue: "heart_rate",
    HKQuantityTypeIdentifier.restingHeartRate.rawValue: "resting_heart_rate",
    HKQuantityTypeIdentifier.walkingHeartRateAverage.rawValue: "walking_heart_rate_average",
    HKQuantityTypeIdentifier.heartRateVariabilitySDNN.rawValue: "heart_rate_variability",
    HKQuantityTypeIdentifier.oxygenSaturation.rawValue: "blood_oxygen_saturation",
    HKQuantityTypeIdentifier.bodyTemperature.rawValue: "body_temperature",
    HKQuantityTypeIdentifier.basalBodyTemperature.rawValue: "basal_body_temperature",
    HKQuantityTypeIdentifier.respiratoryRate.rawValue: "respiratory_rate",
    HKQuantityTypeIdentifier.bloodPressureSystolic.rawValue: "blood_pressure_systolic",
    HKQuantityTypeIdentifier.bloodPressureDiastolic.rawValue: "blood_pressure_diastolic",
    HKQuantityTypeIdentifier.peripheralPerfusionIndex.rawValue: "peripheral_perfusion_index",
    HKQuantityTypeIdentifier.electrodermalActivity.rawValue: "electrodermal_activity",
    HKQuantityTypeIdentifier.heartRateRecoveryOneMinute.rawValue: "heart_rate_recovery_one_minute",
    HKQuantityTypeIdentifier.atrialFibrillationBurden.rawValue: "atrial_fibrillation_burden",
    // Mobility & Fitness
    HKQuantityTypeIdentifier.vo2Max.rawValue: "vo2_max",
    HKQuantityTypeIdentifier.walkingSpeed.rawValue: "walking_speed",
    HKQuantityTypeIdentifier.walkingStepLength.rawValue: "walking_step_length",
    HKQuantityTypeIdentifier.walkingAsymmetryPercentage.rawValue: "walking_asymmetry_percentage",
    HKQuantityTypeIdentifier.walkingDoubleSupportPercentage.rawValue: "walking_double_support_percentage",
    HKQuantityTypeIdentifier.stairAscentSpeed.rawValue: "stair_ascent_speed",
    HKQuantityTypeIdentifier.stairDescentSpeed.rawValue: "stair_descent_speed",
    HKQuantityTypeIdentifier.sixMinuteWalkTestDistance.rawValue: "six_minute_walk_test_distance",
    HKQuantityTypeIdentifier.runningStrideLength.rawValue: "running_stride_length",
    HKQuantityTypeIdentifier.runningVerticalOscillation.rawValue: "running_vertical_oscillation",
    HKQuantityTypeIdentifier.runningGroundContactTime.rawValue: "running_ground_contact_time",
    HKQuantityTypeIdentifier.runningPower.rawValue: "running_power",
    HKQuantityTypeIdentifier.runningSpeed.rawValue: "running_speed",
    HKQuantityTypeIdentifier.appleWalkingSteadiness.rawValue: "apple_walking_steadiness",
    "HKQuantityTypeIdentifierCyclingSpeed": "cycling_speed",
    "HKQuantityTypeIdentifierCyclingPower": "cycling_power",
    "HKQuantityTypeIdentifierCyclingFunctionalThresholdPower": "cycling_functional_threshold_power",
    "HKQuantityTypeIdentifierCyclingCadence": "cycling_cadence",
    // Lab & Clinical
    HKQuantityTypeIdentifier.bloodGlucose.rawValue: "blood_glucose",
    HKQuantityTypeIdentifier.insulinDelivery.rawValue: "insulin_delivery",
    HKQuantityTypeIdentifier.bloodAlcoholContent.rawValue: "blood_alcohol_content",
    HKQuantityTypeIdentifier.numberOfTimesFallen.rawValue: "number_of_times_fallen",
    // Hearing
    HKQuantityTypeIdentifier.environmentalAudioExposure.rawValue: "environmental_audio_exposure",
    HKQuantityTypeIdentifier.headphoneAudioExposure.rawValue: "headphone_audio_exposure",
    // Respiratory
    HKQuantityTypeIdentifier.forcedVitalCapacity.rawValue: "forced_vital_capacity",
    HKQuantityTypeIdentifier.forcedExpiratoryVolume1.rawValue: "forced_expiratory_volume_1",
    HKQuantityTypeIdentifier.peakExpiratoryFlowRate.rawValue: "peak_expiratory_flow_rate",
    HKQuantityTypeIdentifier.inhalerUsage.rawValue: "inhaler_usage",
    HKQuantityTypeIdentifier.uvExposure.rawValue: "uv_exposure",
    // Nutrition
    HKQuantityTypeIdentifier.dietaryEnergyConsumed.rawValue: "dietary_energy_consumed",
    HKQuantityTypeIdentifier.dietaryProtein.rawValue: "dietary_protein",
    HKQuantityTypeIdentifier.dietaryCarbohydrates.rawValue: "dietary_carbohydrates",
    HKQuantityTypeIdentifier.dietaryFatTotal.rawValue: "dietary_fat_total",
    HKQuantityTypeIdentifier.dietaryFatSaturated.rawValue: "dietary_fat_saturated",
    HKQuantityTypeIdentifier.dietaryFatMonounsaturated.rawValue: "dietary_fat_monounsaturated",
    HKQuantityTypeIdentifier.dietaryFatPolyunsaturated.rawValue: "dietary_fat_polyunsaturated",
    HKQuantityTypeIdentifier.dietarySugar.rawValue: "dietary_sugar",
    HKQuantityTypeIdentifier.dietaryFiber.rawValue: "dietary_fiber",
    HKQuantityTypeIdentifier.dietaryCholesterol.rawValue: "dietary_cholesterol",
    HKQuantityTypeIdentifier.dietarySodium.rawValue: "dietary_sodium",
    HKQuantityTypeIdentifier.dietaryCalcium.rawValue: "dietary_calcium",
    HKQuantityTypeIdentifier.dietaryPhosphorus.rawValue: "dietary_phosphorus",
    HKQuantityTypeIdentifier.dietaryMagnesium.rawValue: "dietary_magnesium",
    HKQuantityTypeIdentifier.dietaryPotassium.rawValue: "dietary_potassium",
    HKQuantityTypeIdentifier.dietaryIron.rawValue: "dietary_iron",
    HKQuantityTypeIdentifier.dietaryZinc.rawValue: "dietary_zinc",
    HKQuantityTypeIdentifier.dietaryManganese.rawValue: "dietary_manganese",
    HKQuantityTypeIdentifier.dietaryCopper.rawValue: "dietary_copper",
    HKQuantityTypeIdentifier.dietarySelenium.rawValue: "dietary_selenium",
    HKQuantityTypeIdentifier.dietaryChromium.rawValue: "dietary_chromium",
    HKQuantityTypeIdentifier.dietaryMolybdenum.rawValue: "dietary_molybdenum",
    HKQuantityTypeIdentifier.dietaryIodine.rawValue: "dietary_iodine",
    HKQuantityTypeIdentifier.dietaryVitaminA.rawValue: "dietary_vitamin_a",
    HKQuantityTypeIdentifier.dietaryVitaminB6.rawValue: "dietary_vitamin_b6",
    HKQuantityTypeIdentifier.dietaryVitaminB12.rawValue: "dietary_vitamin_b12",
    HKQuantityTypeIdentifier.dietaryVitaminC.rawValue: "dietary_vitamin_c",
    HKQuantityTypeIdentifier.dietaryVitaminD.rawValue: "dietary_vitamin_d",
    HKQuantityTypeIdentifier.dietaryVitaminE.rawValue: "dietary_vitamin_e",
    HKQuantityTypeIdentifier.dietaryVitaminK.rawValue: "dietary_vitamin_k",
    HKQuantityTypeIdentifier.dietaryThiamin.rawValue: "dietary_thiamin",
    HKQuantityTypeIdentifier.dietaryRiboflavin.rawValue: "dietary_riboflavin",
    HKQuantityTypeIdentifier.dietaryNiacin.rawValue: "dietary_niacin",
    HKQuantityTypeIdentifier.dietaryPantothenicAcid.rawValue: "dietary_pantothenic_acid",
    HKQuantityTypeIdentifier.dietaryFolate.rawValue: "dietary_folate",
    HKQuantityTypeIdentifier.dietaryBiotin.rawValue: "dietary_biotin",
    HKQuantityTypeIdentifier.dietaryCaffeine.rawValue: "dietary_caffeine",
    HKQuantityTypeIdentifier.dietaryWater.rawValue: "dietary_water",
    HKQuantityTypeIdentifier.dietaryChloride.rawValue: "dietary_chloride",
    // Other
    HKQuantityTypeIdentifier.underwaterDepth.rawValue: "underwater_depth",
    HKQuantityTypeIdentifier.waterTemperature.rawValue: "water_temperature",
]

/// HKObject convenience extensions for payload construction.
extension HKObject {
    var sourceDisplayName: String {
        sourceRevision.source.name
    }
    var sourceBundleID: String {
        sourceRevision.source.bundleIdentifier
    }
    var deviceName: String {
        device?.name ?? ""
    }
}

extension HKQuantitySample {
    func jsonMetadata() -> String {
        guard let meta = metadata, !meta.isEmpty else { return "" }
        guard let data = try? JSONSerialization.data(withJSONObject: meta),
              let str = String(data: data, encoding: .utf8) else { return "" }
        return str
    }
}

extension HKElectrocardiogram.Classification {
    var label: String {
        switch self {
        case .notSet: return "Not Set"
        case .sinusRhythm: return "Sinus Rhythm"
        case .atrialFibrillation: return "Atrial Fibrillation"
        case .inconclusiveLowHeartRate: return "Inconclusive Low Heart Rate"
        case .inconclusiveHighHeartRate: return "Inconclusive High Heart Rate"
        case .inconclusivePoorReading: return "Inconclusive Poor Reading"
        case .inconclusiveOther: return "Inconclusive Other"
        case .unrecognized: return "Unrecognized"
        @unknown default: return "Unknown"
        }
    }
}

extension HKWorkout {
    var activityTypeName: String {
        workoutActivityType.name
    }
}

extension HKWorkoutActivityType {
    var name: String {
        switch self {
        case .americanFootball: return "American Football"
        case .archery: return "Archery"
        case .australianFootball: return "Australian Football"
        case .badminton: return "Badminton"
        case .baseball: return "Baseball"
        case .basketball: return "Basketball"
        case .bowling: return "Bowling"
        case .boxing: return "Boxing"
        case .climbing: return "Climbing"
        case .cricket: return "Cricket"
        case .crossTraining: return "Cross Training"
        case .curling: return "Curling"
        case .cycling: return "Cycling"
        case .elliptical: return "Elliptical"
        case .equestrianSports: return "Equestrian Sports"
        case .fencing: return "Fencing"
        case .fishing: return "Fishing"
        case .functionalStrengthTraining: return "Functional Strength Training"
        case .golf: return "Golf"
        case .gymnastics: return "Gymnastics"
        case .handball: return "Handball"
        case .hiking: return "Hiking"
        case .hockey: return "Hockey"
        case .hunting: return "Hunting"
        case .lacrosse: return "Lacrosse"
        case .martialArts: return "Martial Arts"
        case .mindAndBody: return "Mind and Body"
        case .paddleSports: return "Paddle Sports"
        case .play: return "Play"
        case .preparationAndRecovery: return "Preparation and Recovery"
        case .racquetball: return "Racquetball"
        case .rowing: return "Rowing"
        case .rugby: return "Rugby"
        case .running: return "Running"
        case .sailing: return "Sailing"
        case .skatingSports: return "Skating Sports"
        case .snowSports: return "Snow Sports"
        case .soccer: return "Soccer"
        case .softball: return "Softball"
        case .squash: return "Squash"
        case .stairClimbing: return "Stair Climbing"
        case .surfingSports: return "Surfing Sports"
        case .swimming: return "Swimming"
        case .tableTennis: return "Table Tennis"
        case .tennis: return "Tennis"
        case .trackAndField: return "Track and Field"
        case .traditionalStrengthTraining: return "Traditional Strength Training"
        case .volleyball: return "Volleyball"
        case .walking: return "Walking"
        case .waterFitness: return "Water Fitness"
        case .waterPolo: return "Water Polo"
        case .waterSports: return "Water Sports"
        case .wrestling: return "Wrestling"
        case .yoga: return "Yoga"
        case .barre: return "Barre"
        case .coreTraining: return "Core Training"
        case .crossCountrySkiing: return "Cross Country Skiing"
        case .downhillSkiing: return "Downhill Skiing"
        case .flexibility: return "Flexibility"
        case .highIntensityIntervalTraining: return "High Intensity Interval Training"
        case .jumpRope: return "Jump Rope"
        case .kickboxing: return "Kickboxing"
        case .pilates: return "Pilates"
        case .snowboarding: return "Snowboarding"
        case .stairs: return "Stairs"
        case .stepTraining: return "Step Training"
        case .wheelchairWalkPace: return "Wheelchair Walk Pace"
        case .wheelchairRunPace: return "Wheelchair Run Pace"
        case .mixedCardio: return "Mixed Cardio"
        case .handCycling: return "Hand Cycling"
        case .fitnessGaming: return "Fitness Gaming"
        case .dance: return "Dance"
        case .socialDance: return "Social Dance"
        case .pickleball: return "Pickleball"
        case .cooldown: return "Cooldown"
        case .swimBikeRun: return "Swim Bike Run"
        case .transition: return "Transition"
        case .taiChi: return "Tai Chi"
        case .discSports: return "Disc Sports"
        case .cardioDance: return "Cardio Dance"
        case .danceInspiredTraining: return "Dance Inspired Training"
        case .mixedMetabolicCardioTraining: return "Mixed Metabolic Cardio"
        case .underwaterDiving: return "Underwater Diving"
        case .other: return "Other"
        @unknown default: return "Other"
        }
    }
}
