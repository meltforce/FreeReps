import SwiftUI
import UIKit

/// One category as a tile in the dashboard grid, in the style of Apple Health.
/// The category colour is used for the symbol and the name only.
struct CategoryStatusCard: View {
    let state: CategorySyncState
    /// Reference time for the relative "Synced … ago" text; the dashboard advances it.
    let now: Date
    var onReset: (() -> Void)? = nil
    var onSync: (() -> Void)? = nil
    var isSyncRunning: Bool = false

    @State private var showResetConfirm = false

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 5) {
                Image(systemName: state.systemImage)
                    .font(.system(size: 15))
                    .symbolVariant(.fill)
                Text(state.displayName)
                    .font(.system(size: 14, weight: .semibold))
                    .lineLimit(1)
                    .truncationMode(.tail)
            }
            .foregroundStyle(state.tint)

            Text(state.recordCount.formatted())
                .font(.system(size: 24, weight: .bold, design: .rounded))
                .tracking(-0.24)
                .monospacedDigit()
                .lineLimit(1)
                .minimumScaleFactor(0.7)
                .contentTransition(.numericText())
                .animation(.default, value: state.recordCount)

            statusLine
                .frame(height: 16)
        }
        .padding(.vertical, 12)
        .padding(.horizontal, 14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Color(.secondarySystemGroupedBackground), in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .contentShape(.contextMenuPreview, RoundedRectangle(cornerRadius: 16, style: .continuous))
        .contextMenu {
            Button {
                onSync?()
            } label: {
                Label("Sync", systemImage: "arrow.clockwise")
            }
            .disabled(isSyncRunning)
            Button(role: .destructive) {
                showResetConfirm = true
            } label: {
                Label("Reset…", systemImage: "trash")
            }
            .disabled(isSyncRunning)
        }
        .confirmationDialog(
            "Reset \(state.displayName)?",
            isPresented: $showResetConfirm,
            titleVisibility: .visible
        ) {
            Button("Delete All Records", role: .destructive) {
                onReset?()
            }
        } message: {
            Text("This permanently deletes all \(state.displayName) records from the server. This cannot be undone.")
        }
    }

    @ViewBuilder
    private var statusLine: some View {
        switch state.status {
        case .syncing:
            ProgressView(value: state.progressFraction)
                .tint(Color("Brand"))
        case .failed:
            statusLabel("exclamationmark.circle.fill", "Sync failed", color: Color("StatusError"))
        case .completed, .idle:
            if let days = state.daysBehind {
                statusLabel("exclamationmark.circle.fill", days == 1 ? "1 day behind" : "\(days) days behind", color: .orange)
            } else if state.status == .completed {
                statusLabel("checkmark.circle.fill", syncedText, color: .secondary)
            } else {
                statusLabel("circle.dashed", "Not synced", color: .secondary)
            }
        }
    }

    private var syncedText: String {
        guard let date = state.lastSyncDate else { return "Synced" }
        return "Synced \(RelativeTime.describe(date, now: now, style: .abbreviated))"
    }

    private func statusLabel(_ symbol: String, _ text: String, color: Color) -> some View {
        HStack(spacing: 4) {
            Image(systemName: symbol)
                .font(.system(size: 12))
            Text(text)
                .font(.caption)
                .lineLimit(1)
        }
        .foregroundStyle(color)
    }
}

// MARK: - Category colours

extension CategorySyncState {
    /// Apple Health colour of the category; categories without one use the brand colour.
    var tint: Color {
        switch id {
        case "qty_\(HealthCategory.activity.rawValue)", "cat_workouts", "cat_activity_summaries", "cat_strength":
            return .categoryTint(light: 0xF2600C, dark: 0xFF7A2E)
        case "qty_\(HealthCategory.vitals.rawValue)", "cat_bp", "cat_category":
            return .categoryTint(light: 0xE8263C, dark: 0xFF4F62)
        case "qty_\(HealthCategory.sleep.rawValue)", "qty_\(HealthCategory.mindfulness.rawValue)", "cat_state_of_mind":
            return .categoryTint(light: 0x0A9BB0, dark: 0x48D1E0)
        case "qty_\(HealthCategory.body.rawValue)":
            return .categoryTint(light: 0x9B3FD4, dark: 0xC27BF0)
        case "qty_\(HealthCategory.mobility.rawValue)":
            return .categoryTint(light: 0xE08600, dark: 0xFFA630)
        case "qty_\(HealthCategory.nutrition.rawValue)":
            return .categoryTint(light: 0x24A148, dark: 0x3ED36A)
        case "qty_\(HealthCategory.bloodPressure.rawValue)":
            return .categoryTint(light: 0xE8263C, dark: 0xFF4F62)
        case "cat_workout_routes":
            return .categoryTint(light: 0x0A84FF, dark: 0x409CFF)
        default:
            return Color("Brand")
        }
    }
}

private extension Color {
    static func categoryTint(light: UInt32, dark: UInt32) -> Color {
        Color(UIColor { traits in
            UIColor(rgb: traits.userInterfaceStyle == .dark ? dark : light)
        })
    }
}

private extension UIColor {
    convenience init(rgb: UInt32) {
        self.init(
            red: CGFloat((rgb >> 16) & 0xFF) / 255,
            green: CGFloat((rgb >> 8) & 0xFF) / 255,
            blue: CGFloat(rgb & 0xFF) / 255,
            alpha: 1
        )
    }
}
