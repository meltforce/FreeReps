import Foundation

struct FreeRepsConfig: Codable, Equatable {
    var host: String
    var port: UInt16
    var useHTTPS: Bool = true
    /// Max years of HealthKit history to backfill. nil = all data (back to 2000).
    var backfillYears: Int? = 2

    static let `default` = FreeRepsConfig(
        host: "freereps",
        port: 443,
        useHTTPS: true,
        backfillYears: 2
    )

    var baseURL: URL {
        let scheme = useHTTPS ? "https" : "http"
        return URL(string: "\(scheme)://\(host):\(port)")!
    }

    /// Earliest date to backfill from, based on `backfillYears`.
    var backfillStartDate: Date {
        if let years = backfillYears {
            return Calendar.current.date(byAdding: .year, value: -years, to: Date()) ?? Date()
        }
        return Calendar.current.date(from: DateComponents(year: 2000, month: 1, day: 1))!
    }

    private static let userDefaultsKey = "freerepsConfig_v1"

    static func load() -> FreeRepsConfig {
        guard let data = UserDefaults.standard.data(forKey: userDefaultsKey),
              let config = try? JSONDecoder().decode(FreeRepsConfig.self, from: data) else {
            return .default
        }
        return config
    }

    func save() {
        if let data = try? JSONEncoder().encode(self) {
            UserDefaults.standard.set(data, forKey: FreeRepsConfig.userDefaultsKey)
        }
        Task { @MainActor in iCloudSyncService.shared.pushFreeRepsConfig(self) }
    }
}
