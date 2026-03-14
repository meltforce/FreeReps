import Foundation
import HealthKit

enum ScanDepth: String, CaseIterable, Identifiable {
    case quick = "Quick"
    case deep = "Deep"
    var id: String { rawValue }
}

struct DeepStats {
    var missing: Int = 0
    var corrupted: Int = 0
    var fixed: Int = 0
}

struct ValidationResult: Identifiable {
    let id: String
    let displayName: String
    let systemImage: String
    let depth: ScanDepth
    // Quick mode
    let hkCount: Int
    let dbCount: Int
    /// When true, HK count is approximate (aggregated types produce fewer DB rows than HK samples).
    var isApproximate: Bool = false
    // Deep mode
    var deepStats: DeepStats?

    var isInSync: Bool {
        if let ds = deepStats { return ds.missing == 0 && ds.corrupted == 0 }
        if isApproximate { return dbCount > 0 || hkCount == 0 }
        return hkCount == dbCount
    }
    var missingCount: Int {
        if isApproximate { return 0 }
        return deepStats?.missing ?? max(0, hkCount - dbCount)
    }
    var corruptedCount: Int { deepStats?.corrupted ?? 0 }
    var fixedCount: Int { deepStats?.fixed ?? 0 }
    var totalIssues: Int { missingCount + corruptedCount }
}

@MainActor
final class DataValidationViewModel: ObservableObject {

    @Published var results: [ValidationResult] = []
    @Published var isValidating = false
    @Published var validationDate: Date?
    @Published var errorMessage: String?
    @Published var progress: Int = 0
    @Published var progressTotal: Int = 0
    @Published var currentScanDetail: String = ""
    @Published var scanDepth: ScanDepth = .quick
    @Published var autoFix: Bool = false
    @Published var repairingCategoryID: String?

    let syncViewModel: SyncViewModel
    private var validationTask: Task<Void, Never>?

    init(syncViewModel: SyncViewModel) {
        self.syncViewModel = syncViewModel
    }

    var outOfSyncCount: Int { results.filter { !$0.isInSync }.count }
    var totalMissing: Int { results.map(\.missingCount).reduce(0, +) }
    var totalCorrupted: Int { results.map(\.corruptedCount).reduce(0, +) }

    // MARK: - Public actions

    func runValidation() {
        guard !isValidating, !syncViewModel.isAnySyncRunning else { return }
        validationTask = Task { await performValidation() }
    }

    func cancelValidation() {
        validationTask?.cancel()
        validationTask = nil
    }

    /// Repair a specific category by re-syncing it.
    func repairCategory(_ categoryID: String) {
        guard !isValidating else { return }
        syncViewModel.repairCategories(categoryIDs: [categoryID])
    }

    func repairAllMissing() {
        let outOfSync = results.filter { !$0.isInSync }.map(\.id)
        guard !outOfSync.isEmpty, !isValidating else { return }
        syncViewModel.repairCategories(categoryIDs: outOfSync)
    }

    // MARK: - Validation

    private func performValidation() async {
        isValidating = true
        errorMessage = nil
        results = []
        progress = 0
        progressTotal = syncViewModel.categories.count
        currentScanDetail = ""
        defer {
            isValidating = false
            currentScanDetail = ""
        }

        var built: [ValidationResult] = []
        for catState in syncViewModel.categories {
            guard !Task.isCancelled else { break }
            currentScanDetail = catState.displayName

            let hkCount = await countHealthKit(for: catState.id)
            let approximate = categoryHasAggregatedTypes(catState.id)
            let result = ValidationResult(
                id: catState.id,
                displayName: catState.displayName,
                systemImage: catState.systemImage,
                depth: .quick,
                hkCount: hkCount,
                dbCount: catState.recordCount,
                isApproximate: approximate
            )
            built.append(result)
            progress += 1
        }

        if !Task.isCancelled {
            results = built.sorted { lhs, rhs in
                if lhs.isInSync != rhs.isInSync { return !lhs.isInSync }
                return lhs.displayName < rhs.displayName
            }
            validationDate = Date()
        }
    }

    /// Returns true if the category contains types that use on-device aggregation,
    /// meaning HealthKit sample count won't match DB bucket count.
    private func categoryHasAggregatedTypes(_ categoryID: String) -> Bool {
        guard categoryID.hasPrefix("qty_") else { return false }
        let rawCat = String(categoryID.dropFirst(4))
        guard let cat = HealthCategory.allCases.first(where: { $0.rawValue == rawCat }),
              let types = HealthDataTypes.quantityTypesByCategory.first(where: { $0.0 == cat })?.1
        else { return false }
        return types.contains { typeDesc in
            switch typeDesc.syncStrategy {
            case .aggregate, .aggregateCumulative: return true
            case .individual: return false
            }
        }
    }

    // MARK: - HealthKit counts

    private func countHealthKit(for categoryID: String) async -> Int {
        if categoryID.hasPrefix("qty_") {
            let rawCat = String(categoryID.dropFirst(4))
            guard let cat = HealthCategory.allCases.first(where: { $0.rawValue == rawCat }),
                  let types = HealthDataTypes.quantityTypesByCategory.first(where: { $0.0 == cat })?.1
            else { return 0 }
            var total = 0
            for typeDesc in types {
                guard let hkType = HKObjectType.quantityType(forIdentifier: typeDesc.hkIdentifier) else { continue }
                total += await HealthKitService.shared.sampleCount(for: hkType)
            }
            return total
        }
        switch categoryID {
        case "cat_category":
            var total = 0
            for typeDesc in HealthDataTypes.allCategoryTypes {
                guard let hkType = HKObjectType.categoryType(forIdentifier: typeDesc.hkIdentifier) else { continue }
                total += await HealthKitService.shared.sampleCount(for: hkType)
            }
            return total
        case "cat_workouts":
            return await HealthKitService.shared.sampleCount(for: .workoutType())
        case "cat_bp":
            guard let t = HKObjectType.correlationType(forIdentifier: .bloodPressure) else { return 0 }
            return await HealthKitService.shared.sampleCount(for: t)
        case "cat_ecg":
            return await HealthKitService.shared.sampleCount(for: .electrocardiogramType())
        case "cat_audiogram":
            return await HealthKitService.shared.sampleCount(for: .audiogramSampleType())
        case "cat_activity_summaries":
            return await HealthKitService.shared.countActivitySummaries()
        case "cat_workout_routes":
            return await HealthKitService.shared.sampleCount(for: HKSeriesType.workoutRoute())
        case "cat_medications":
            if #available(iOS 26, *) {
                return await HealthKitService.shared.countMedicationDoseEvents()
            }
            return 0
        case "cat_vision":
            return await HealthKitService.shared.sampleCount(for: .visionPrescriptionType())
        case "cat_state_of_mind":
            if #available(iOS 18, *) {
                return await HealthKitService.shared.sampleCount(for: .stateOfMindType())
            }
            return 0
        default:
            return 0
        }
    }
}
