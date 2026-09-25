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
        .init(id: HKQuantityTypeIdentifier.basalEnergyBurned.rawValue, displayName: "Resting Energy", category: .activity, unit: .kilocalorie(), unitString: "kcal", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.activeEnergyBurned.rawValue, displayName: "Active Energy", category: .activity, unit: .kilocalorie(), unitString: "kcal", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.flightsClimbed.rawValue, displayName: "Flights Climbed", category: .activity, unit: .count(), unitString: "count", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.appleExerciseTime.rawValue, displayName: "Exercise Minutes", category: .activity, unit: .minute(), unitString: "min", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.appleMoveTime.rawValue, displayName: "Move Time", category: .activity, unit: .minute(), unitString: "min", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.appleStandTime.rawValue, displayName: "Stand Time", category: .activity, unit: .minute(), unitString: "min", syncStrategy: .aggregateCumulative(interval: 3600)),
        .init(id: HKQuantityTypeIdentifier.swimmingStrokeCount.rawValue, displayName: "Swimming Strokes", category: .activity, unit: .count(), unitString: "count", syncStrategy: .aggregateCumulative(interval: 3600)),
        // Body
        .init(id: HKQuantityTypeIdentifier.bodyMass.rawValue, displayName: "Weight", category: .body, unit: .gramUnit(with: .kilo), unitString: "kg"),
        .init(id: HKQuantityTypeIdentifier.bodyFatPercentage.rawValue, displayName: "Body Fat %", category: .body, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.leanBodyMass.rawValue, displayName: "Lean Body Mass", category: .body, unit: .gramUnit(with: .kilo), unitString: "kg"),
        // Vitals
        .init(id: HKQuantityTypeIdentifier.heartRate.rawValue, displayName: "Heart Rate", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "bpm"),
        .init(id: HKQuantityTypeIdentifier.restingHeartRate.rawValue, displayName: "Resting Heart Rate", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "bpm"),
        .init(id: HKQuantityTypeIdentifier.walkingHeartRateAverage.rawValue, displayName: "Walking Heart Rate", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "bpm"),
        .init(id: HKQuantityTypeIdentifier.heartRateVariabilitySDNN.rawValue, displayName: "HRV (SDNN)", category: .vitals, unit: .secondUnit(with: .milli), unitString: "ms"),
        .init(id: HKQuantityTypeIdentifier.oxygenSaturation.rawValue, displayName: "Blood Oxygen", category: .vitals, unit: .percent(), unitString: "%"),
        .init(id: HKQuantityTypeIdentifier.bodyTemperature.rawValue, displayName: "Body Temperature", category: .vitals, unit: .degreeCelsius(), unitString: "°C"),
        .init(id: HKQuantityTypeIdentifier.respiratoryRate.rawValue, displayName: "Respiratory Rate", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "breaths/min"),
        .init(id: HKQuantityTypeIdentifier.bloodPressureSystolic.rawValue, displayName: "Systolic BP", category: .bloodPressure, unit: .millimeterOfMercury(), unitString: "mmHg"),
        .init(id: HKQuantityTypeIdentifier.bloodPressureDiastolic.rawValue, displayName: "Diastolic BP", category: .bloodPressure, unit: .millimeterOfMercury(), unitString: "mmHg"),
        // Mobility & Fitness
        .init(id: HKQuantityTypeIdentifier.vo2Max.rawValue, displayName: "VO2 Max", category: .mobility, unit: HKUnit.literUnit(with: .milli).unitDivided(by: HKUnit.gramUnit(with: .kilo).unitMultiplied(by: HKUnit.minute())), unitString: "mL/kg·min"),
        // Nutrition
        .init(id: HKQuantityTypeIdentifier.dietaryCaffeine.rawValue, displayName: "Caffeine", category: .nutrition, unit: .gram(), unitString: "g"),
        .init(id: HKQuantityTypeIdentifier.dietaryWater.rawValue, displayName: "Water", category: .nutrition, unit: .liter(), unitString: "L"),
        // iOS 16+
        .init(id: HKQuantityTypeIdentifier.heartRateRecoveryOneMinute.rawValue, displayName: "Heart Rate Recovery", category: .vitals, unit: HKUnit(from: "count/min"), unitString: "bpm"),
        .init(id: HKQuantityTypeIdentifier.appleSleepingWristTemperature.rawValue, displayName: "Sleeping Wrist Temperature", category: .body, unit: .degreeCelsius(), unitString: "°C"),
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
        .init(id: HKCategoryTypeIdentifier.mindfulSession.rawValue, displayName: "Mindful Session", category: .mindfulness, valueLabels: [0: "Present"]),
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
        types.insert(HKSeriesType.workoutRoute())
        types.insert(HKObjectType.activitySummaryType())
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
        if type == HKSeriesType.workoutRoute() { return "map.fill" }
        if type == HKObjectType.activitySummaryType() { return "chart.bar.fill" }
        return "heart.fill"
    }

    // MARK: Grouping

    /// Quantity types grouped for sync and display. Systolic and diastolic are left
    /// out: they stay in allQuantityTypes because reading the blood pressure
    /// correlation needs their permission, but the correlation sync ("cat_bp")
    /// sends them, and a second path showed Blood Pressure twice on the dashboard.
    static var quantityTypesByCategory: [(HealthCategory, [QuantityTypeDescriptor])] {
        var map: [HealthCategory: [QuantityTypeDescriptor]] = [:]
        for t in allQuantityTypes where t.category != .bloodPressure {
            map[t.category, default: []].append(t)
        }
        return HealthCategory.allCases.compactMap { cat in
            guard let types = map[cat], !types.isEmpty else { return nil }
            return (cat, types)
        }
    }
}
