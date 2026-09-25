import Foundation

/// The deep link the widget opens: `freereps://sync` brings the app to the
/// foreground and starts a sync there, where HealthKit is readable because the
/// device is unlocked.
enum SyncLink {
    static let scheme = "freereps"
    static let syncHost = "sync"
    static let url = URL(string: "\(scheme)://\(syncHost)")!
}

/// "3 minutes ago" for a past date, and "just now" within the first minute:
/// RelativeDateTimeFormatter renders a zero interval as "in 0 seconds", and the
/// dashboard's clock ticks once a minute, so a sync that just ended would read
/// as lying in the future.
enum RelativeTime {
    static func describe(_ date: Date, now: Date, style: RelativeDateTimeFormatter.UnitsStyle) -> String {
        if now.timeIntervalSince(date) < 60 { return "just now" }
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = style
        return formatter.localizedString(for: date, relativeTo: now)
    }
}

/// Outcome of the most recent sync, written by the app and read by the widget.
/// Lives in the App Group's UserDefaults because the widget runs in its own process.
struct LastSyncRecord: Codable, Equatable {
    enum Outcome: String, Codable {
        case succeeded, failed, cancelled
    }

    let finishedAt: Date
    let outcome: Outcome
    /// Error text for a failed run; nil otherwise.
    let message: String?

    static let appGroup = "group.com.meltforce.freereps"
    private static let key = "lastSyncRecord"

    static func load() -> LastSyncRecord? {
        guard let data = UserDefaults(suiteName: appGroup)?.data(forKey: key) else { return nil }
        return try? JSONDecoder().decode(LastSyncRecord.self, from: data)
    }

    func save() {
        guard let data = try? JSONEncoder().encode(self) else { return }
        UserDefaults(suiteName: Self.appGroup)?.set(data, forKey: Self.key)
    }
}
