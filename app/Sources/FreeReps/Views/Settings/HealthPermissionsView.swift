import HealthKit
import SwiftUI

struct HealthPermissionsView: View {
    @ObservedObject var vm: SettingsViewModel

    var body: some View {
        List {
            // Status banner
            Section {
                VStack(spacing: 12) {
                    HStack(spacing: 14) {
                        Image(systemName: statusIcon)
                            .font(.system(size: 36))
                            .foregroundStyle(statusColor)
                        VStack(alignment: .leading, spacing: 3) {
                            Text(statusTitle)
                                .font(.headline)
                            Text(statusSubtitle)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)

                    if vm.isRequestingPermissions {
                        HStack(spacing: 10) {
                            ProgressView()
                            Text("Requesting permissions…")
                                .font(.subheadline)
                                .foregroundStyle(.secondary)
                        }
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 10)
                    } else if !vm.permissionsRequested {
                        actionButton(
                            label: "Request All Permissions",
                            icon: "checkmark.shield.fill",
                            color: .green
                        ) {
                            vm.requestAllPermissions()
                        }
                    } else {
                        if !vm.deniedTypes.isEmpty {
                            actionButton(
                                label: "Request Missing Permissions (\(vm.deniedTypes.count))",
                                icon: "exclamationmark.arrow.circlepath",
                                color: .orange
                            ) {
                                vm.requestMissingPermissions()
                            }
                        }

                        actionButton(
                            label: "Re-request All Permissions",
                            icon: "arrow.clockwise.circle",
                            color: Color.secondary.opacity(0.15),
                            foreground: .secondary
                        ) {
                            vm.requestAllPermissions()
                        }
                    }

                    // Always show the Health app button once permissions have been requested
                    if vm.permissionsRequested {
                        actionButton(
                            label: "Open Health App to Change Permissions",
                            icon: "heart.fill",
                            color: Color.red.opacity(0.12),
                            foreground: .red
                        ) {
                            if let url = URL(string: "x-apple-health://") {
                                UIApplication.shared.open(url)
                            }
                        }
                    }
                }
                .padding(.vertical, 4)
            }

            // Error banner
            if let err = vm.errorMessage {
                Section {
                    HStack(spacing: 10) {
                        Image(systemName: "exclamationmark.triangle.fill")
                            .foregroundStyle(.red)
                        Text(err)
                            .font(.caption)
                            .foregroundStyle(.red)
                            .lineLimit(3)
                    }
                    .padding(8)
                    .background(Color.red.opacity(0.08), in: RoundedRectangle(cornerRadius: 8))
                    .listRowBackground(Color.clear)
                    .listRowInsets(.init())
                }
            }

            // Per-object authorization (always shown — these require a separate picker each time)
            Section("Individual Item Access") {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Some data types require you to individually select which items to share. Tap each button below to choose which items FreeReps can access.")
                        .font(.caption)
                        .foregroundStyle(.secondary)

                    actionButton(
                        label: "Authorize Vision Prescriptions",
                        icon: "eye.fill",
                        color: Color.teal.opacity(0.12),
                        foreground: .teal
                    ) {
                        vm.requestVisionPrescriptionAccess()
                    }

                    if #available(iOS 26, *) {
                        actionButton(
                            label: "Authorize Medications",
                            icon: "pills.fill",
                            color: Color.purple.opacity(0.12),
                            foreground: .purple
                        ) {
                            vm.requestMedicationAccess()
                        }
                    }
                }
                .padding(.vertical, 4)
            }

            // iOS permission dialog limitation notice
            if vm.permissionsRequested {
                Section {
                    VStack(alignment: .leading, spacing: 6) {
                        Label("iOS Permission Dialog Limitation", systemImage: "info.circle.fill")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(.orange)
                        Text("iOS only shows the Health permission dialog once per type. If tapping \"Re-request\" shows no dialog, your permissions are already set. To change access, open the Health app → Sharing → Apps → FreeReps.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    .padding(.vertical, 2)
                }
            }

            // Denied / missing permissions (surfaced to top)
            if !vm.deniedTypes.isEmpty {
                Section {
                    ForEach(vm.deniedTypes.sorted(by: { HealthDataTypes.displayName(for: $0) < HealthDataTypes.displayName(for: $1) }), id: \.identifier) { type in
                        permissionRow(
                            name: HealthDataTypes.displayName(for: type),
                            icon: HealthDataTypes.systemImage(for: type),
                            status: .denied
                        )
                    }
                } header: {
                    HStack(spacing: 6) {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundStyle(.red)
                        Text("Not Yet Requested (\(vm.deniedTypes.count))")
                    }
                }
            }

            // Granted permissions
            if !vm.grantedTypes.isEmpty {
                Section {
                    ForEach(vm.grantedTypes.sorted(by: { HealthDataTypes.displayName(for: $0) < HealthDataTypes.displayName(for: $1) }), id: \.identifier) { type in
                        permissionRow(
                            name: HealthDataTypes.displayName(for: type),
                            icon: HealthDataTypes.systemImage(for: type),
                            status: .granted
                        )
                    }
                } header: {
                    HStack(spacing: 6) {
                        Image(systemName: "checkmark.circle.fill")
                            .foregroundStyle(.green)
                        Text("Granted (\(vm.grantedTypes.count))")
                    }
                }
            }

            // Explanation note
            Section {
                VStack(alignment: .leading, spacing: 6) {
                    Label("About permission status", systemImage: "info.circle")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.secondary)
                    Text("iOS hides whether individual read permissions were granted or denied — this is by design. \"Processed\" means the HealthKit dialog was shown for that type. To change access, go to: Health app > Sharing > Apps > FreeReps.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 2)
            }
        }
        .navigationTitle("Health Permissions")
        .onAppear { vm.refreshPermissionsState() }
    }

    // MARK: - Status helpers

    private var statusIcon: String {
        if !vm.permissionsRequested { return "shield.slash.fill" }
        if vm.deniedTypes.isEmpty { return "checkmark.shield.fill" }
        return "exclamationmark.shield.fill"
    }

    private var statusColor: Color {
        if !vm.permissionsRequested { return .orange }
        if vm.deniedTypes.isEmpty { return .green }
        return .orange
    }

    private var statusTitle: String {
        if !vm.permissionsRequested { return "Permissions Not Yet Requested" }
        if vm.deniedTypes.isEmpty { return "All Permissions Granted" }
        return "\(vm.deniedTypes.count) Permission(s) Not Yet Requested"
    }

    private var statusSubtitle: String {
        if !vm.permissionsRequested {
            return "Tap below to request access to all health data types."
        }
        if vm.deniedTypes.isEmpty {
            return "FreeReps has access to all requested data types."
        }
        return "Some types haven't been requested yet. Tap below to present the HealthKit dialog."
    }

    // MARK: - Subviews

    private enum PermissionStatus {
        case granted, denied
    }

    private func actionButton(
        label: String,
        icon: String,
        color: Color,
        foreground: Color = .white,
        action: @escaping () -> Void
    ) -> some View {
        Button(action: action) {
            Label(label, systemImage: icon)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 10)
                .background(color, in: RoundedRectangle(cornerRadius: 10))
                .foregroundStyle(foreground)
                .font(.subheadline.weight(.semibold))
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    private func permissionRow(name: String, icon: String, status: PermissionStatus) -> some View {
        HStack {
            Image(systemName: icon)
                .foregroundStyle(status == .granted ? .blue : .red)
                .frame(width: 20)
            Text(name)
                .font(.subheadline)
            Spacer()
            Image(systemName: status == .granted ? "checkmark.circle.fill" : "xmark.circle.fill")
                .foregroundStyle(status == .granted ? .green : .red)
        }
    }
}
