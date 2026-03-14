import SwiftUI

struct DataValidationView: View {

    @StateObject private var vm: DataValidationViewModel
    @ObservedObject private var syncVM: SyncViewModel

    init(syncViewModel: SyncViewModel) {
        _syncVM = ObservedObject(wrappedValue: syncViewModel)
        _vm = StateObject(wrappedValue: DataValidationViewModel(syncViewModel: syncViewModel))
    }

    var body: some View {
        List {
            configSection
            progressSection
            if !vm.results.isEmpty {
                summarySection
                resultsSection
            }
            if let error = vm.errorMessage {
                Section {
                    Label(error, systemImage: "exclamationmark.triangle.fill")
                        .foregroundStyle(.red)
                        .font(.subheadline)
                }
            }
        }
        .navigationTitle("Data Validation")
        .navigationBarTitleDisplayMode(.inline)
    }

    // MARK: - Config section

    @ViewBuilder
    private var configSection: some View {
        Section {
            Text("Compares every HealthKit category against the remote database. Quick mode checks record counts; Deep mode verifies every record's UUID and values across your full history.")
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .padding(.vertical, 4)

            Picker("Scan Depth", selection: $vm.scanDepth) {
                ForEach(ScanDepth.allCases) { depth in
                    Text(depth.rawValue).tag(depth)
                }
            }
            .pickerStyle(.segmented)
            .disabled(vm.isValidating || syncVM.isAnySyncRunning)

            if vm.scanDepth == .deep {
                Toggle(isOn: $vm.autoFix) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Fix Issues Automatically")
                            .font(.subheadline)
                        Text("Writes missing and corrupted quantity/category records to the database during the scan")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                .disabled(vm.isValidating || syncVM.isAnySyncRunning)
            }

            if let date = vm.validationDate {
                Text("Last checked \(date.formatted(date: .abbreviated, time: .shortened))")
                    .font(.caption)
                    .foregroundStyle(.tertiary)
            }
        }
    }

    // MARK: - Progress / action section

    @ViewBuilder
    private var progressSection: some View {
        Section {
            if vm.isValidating {
                VStack(alignment: .leading, spacing: 8) {
                    HStack(spacing: 12) {
                        ProgressView()
                        VStack(alignment: .leading, spacing: 2) {
                            Text(vm.repairingCategoryID != nil ? "Repairing…" : "Scanning…")
                                .font(.subheadline)
                            if vm.progressTotal > 0 {
                                Text("\(vm.progress) of \(vm.progressTotal) categories")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                        Spacer()
                        Button("Cancel") {
                            vm.cancelValidation()
                        }
                        .font(.subheadline)
                        .foregroundStyle(.red)
                    }
                    if !vm.currentScanDetail.isEmpty {
                        Text(vm.currentScanDetail)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                            .truncationMode(.middle)
                    }
                }
                .padding(.vertical, 4)
            } else {
                Button {
                    Task { vm.runValidation() }
                } label: {
                    Label("Run Validation", systemImage: "checkmark.shield")
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 8)
                        .background(
                            syncVM.isAnySyncRunning ? Color.secondary.opacity(0.15) : Color.blue,
                            in: RoundedRectangle(cornerRadius: 10)
                        )
                        .foregroundStyle(syncVM.isAnySyncRunning ? Color.secondary : Color.white)
                        .font(.subheadline.weight(.semibold))
                }
                .buttonStyle(.plain)
                .disabled(syncVM.isAnySyncRunning)
            }
        }
    }

    // MARK: - Summary section

    @ViewBuilder
    private var summarySection: some View {
        Section("Summary") {
            if vm.outOfSyncCount == 0 {
                HStack(spacing: 10) {
                    Image(systemName: "checkmark.shield.fill")
                        .foregroundStyle(.green)
                        .font(.title2)
                    VStack(alignment: .leading, spacing: 2) {
                        Text("All data in sync")
                            .font(.subheadline.weight(.semibold))
                        Text("Every HealthKit record is present in the database.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                .padding(.vertical, 4)
            } else {
                HStack(spacing: 10) {
                    Image(systemName: "exclamationmark.shield.fill")
                        .foregroundStyle(.orange)
                        .font(.title2)
                    VStack(alignment: .leading, spacing: 2) {
                        Text("\(vm.outOfSyncCount) categor\(vm.outOfSyncCount == 1 ? "y" : "ies") out of sync")
                            .font(.subheadline.weight(.semibold))
                        summaryDetail
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                .padding(.vertical, 4)

                Button {
                    vm.repairAllMissing()
                } label: {
                    Label("Repair All", systemImage: "arrow.clockwise.icloud")
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 8)
                        .background(Color.orange, in: RoundedRectangle(cornerRadius: 10))
                        .foregroundStyle(.white)
                        .font(.subheadline.weight(.semibold))
                }
                .buttonStyle(.plain)
                .disabled(vm.isValidating || syncVM.isAnySyncRunning)
            }
        }
    }

    @ViewBuilder
    private var summaryDetail: some View {
        let missing = vm.totalMissing
        let corrupted = vm.totalCorrupted
        if missing > 0 && corrupted > 0 {
            Text("\(missing.formatted()) missing · \(corrupted.formatted()) corrupted")
        } else if missing > 0 {
            Text("\(missing.formatted()) record\(missing == 1 ? "" : "s") missing from the database")
        } else if corrupted > 0 {
            Text("\(corrupted.formatted()) record\(corrupted == 1 ? "" : "s") with mismatched values")
        } else {
            Text("Issues detected")
        }
    }

    // MARK: - Results section

    private var resultsSection: some View {
        Section("Categories") {
            ForEach(vm.results) { result in
                resultRow(result)
            }
        }
    }

    private func resultRow(_ result: ValidationResult) -> some View {
        HStack(spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 8)
                    .fill(result.isInSync ? Color.green : Color.orange)
                    .frame(width: 36, height: 36)
                Image(systemName: result.systemImage)
                    .font(.system(size: 16))
                    .foregroundStyle(.white)
            }

            VStack(alignment: .leading, spacing: 2) {
                Text(result.displayName)
                    .font(.subheadline.weight(.semibold))
                resultDetail(result)
                    .font(.caption)
            }

            Spacer()

            trailingControl(result)
        }
        .padding(.vertical, 2)
    }

    @ViewBuilder
    private func resultDetail(_ result: ValidationResult) -> some View {
        if result.isInSync {
            if result.depth == .deep {
                Text("\(result.deepStats.map { $0.fixed > 0 ? "\($0.fixed.formatted()) fixed · " : "" } ?? "")All records verified")
                    .foregroundStyle(.secondary)
            } else {
                Text("\(result.dbCount.formatted()) records in sync")
                    .foregroundStyle(.secondary)
            }
        } else if result.depth == .deep, let ds = result.deepStats {
            HStack(spacing: 4) {
                if ds.missing > 0 {
                    Text("\(ds.missing.formatted()) missing")
                        .foregroundStyle(.orange)
                        .fontWeight(.medium)
                }
                if ds.missing > 0 && ds.corrupted > 0 {
                    Text("·").foregroundStyle(.tertiary)
                }
                if ds.corrupted > 0 {
                    Text("\(ds.corrupted.formatted()) corrupted")
                        .foregroundStyle(.red)
                        .fontWeight(.medium)
                }
                if ds.fixed > 0 {
                    Text("·").foregroundStyle(.tertiary)
                    Text("\(ds.fixed.formatted()) fixed")
                        .foregroundStyle(.green)
                }
            }
        } else {
            HStack(spacing: 4) {
                Text("HK: \(result.hkCount.formatted())")
                    .foregroundStyle(.secondary)
                Text("·").foregroundStyle(.tertiary)
                Text("DB: \(result.dbCount.formatted())")
                    .foregroundStyle(.secondary)
                Text("·").foregroundStyle(.tertiary)
                Text("\(result.missingCount.formatted()) missing")
                    .foregroundStyle(.orange)
                    .fontWeight(.medium)
            }
        }
    }

    @ViewBuilder
    private func trailingControl(_ result: ValidationResult) -> some View {
        if result.isInSync {
            Image(systemName: "checkmark.circle.fill")
                .foregroundStyle(.green)
        } else if vm.repairingCategoryID == result.id {
            ProgressView()
                .frame(width: 44)
        } else {
            Button {
                vm.repairCategory(result.id)
            } label: {
                Text("Repair")
                    .font(.caption.weight(.semibold))
                    .padding(.horizontal, 10)
                    .padding(.vertical, 5)
                    .background(Color.orange.opacity(0.15), in: RoundedRectangle(cornerRadius: 6))
                    .foregroundStyle(.orange)
            }
            .buttonStyle(.plain)
            .disabled(vm.isValidating || syncVM.isAnySyncRunning)
        }
    }
}
