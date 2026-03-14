import ActivityKit
import Foundation

// Mirror of the main app's SyncActivityAttributes.
// Both targets compile this same struct so ActivityKit can match
// the live activity started by the app with this widget's configuration.
struct SyncActivityAttributes: ActivityAttributes {
    struct ContentState: Codable, Hashable {
        var phase: String
        var operation: String
        var recordsInserted: Int
        var isFullSync: Bool
    }
}
