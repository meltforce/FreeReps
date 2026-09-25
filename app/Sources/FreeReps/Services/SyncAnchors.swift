import Foundation
import HealthKit

/// HealthKit anchors of the incremental sync, one per sample type, in UserDefaults.
/// An anchor is saved only after the server confirmed every range it covers, so an
/// interrupted sync starts again from the previous anchor and loses nothing.
enum SyncAnchors {
    private static let anchorsKey = "syncAnchors_v1"
    private static let dailyResendKey = "syncAnchorsLastDailyResend_v1"

    static func load() -> [String: HKQueryAnchor] {
        guard let stored = UserDefaults.standard.dictionary(forKey: anchorsKey) as? [String: Data] else { return [:] }
        return stored.compactMapValues {
            try? NSKeyedUnarchiver.unarchivedObject(ofClass: HKQueryAnchor.self, from: $0)
        }
    }

    static func save(_ anchor: HKQueryAnchor, for typeID: String) {
        guard let data = try? NSKeyedArchiver.archivedData(withRootObject: anchor, requiringSecureCoding: true) else { return }
        var stored = UserDefaults.standard.dictionary(forKey: anchorsKey) as? [String: Data] ?? [:]
        stored[typeID] = data
        UserDefaults.standard.set(stored, forKey: anchorsKey)
    }

    /// When the last 24 hours were last re-sent in full, as a safety net for changes
    /// an anchor does not report.
    static var lastDailyResend: Date? {
        get { UserDefaults.standard.object(forKey: dailyResendKey) as? Date }
        set { UserDefaults.standard.set(newValue, forKey: dailyResendKey) }
    }

    /// Called by "Reset Sync State": the next sync is a backfill again.
    static func clear() {
        UserDefaults.standard.removeObject(forKey: anchorsKey)
        UserDefaults.standard.removeObject(forKey: dailyResendKey)
    }
}
