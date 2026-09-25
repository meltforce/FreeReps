import Foundation

struct FreeRepsConfig: Codable, Equatable {
    var host: String
    var port: UInt16
    var useHTTPS: Bool = true
    var testMode: Bool = false
    var testHost: String = ""
    var testPort: UInt16 = 443
    /// Max days of HealthKit history to backfill. nil = all data (back to 2000).
    /// Legacy: `backfillMonths` and `backfillYears` are decoded and converted to days.
    var backfillDays: Int? = 730

    init(host: String, port: UInt16, useHTTPS: Bool = true, testMode: Bool = false, testHost: String = "", testPort: UInt16 = 443, backfillDays: Int? = 730) {
        self.host = host
        self.port = port
        self.useHTTPS = useHTTPS
        self.testMode = testMode
        self.testHost = testHost
        self.testPort = testPort
        self.backfillDays = backfillDays
    }

    static let `default` = FreeRepsConfig(
        host: "freereps.your-tailnet.ts.net",
        port: 443,
        useHTTPS: true,
        testMode: false,
        testHost: "",
        testPort: 443,
        backfillDays: 730
    )

    var baseURL: URL {
        let effectiveHost: String
        let effectivePort: UInt16
        if testMode {
            effectiveHost = testHost
            effectivePort = testPort
        } else {
            effectiveHost = host
            effectivePort = port
        }
        let scheme = useHTTPS ? "https" : "http"
        return URL(string: "\(scheme)://\(effectiveHost):\(effectivePort)")!
    }

    /// Earliest date to backfill from, based on `backfillDays`.
    var backfillStartDate: Date {
        if let days = backfillDays {
            return Calendar.current.date(byAdding: .day, value: -days, to: Date()) ?? Date()
        }
        return Calendar.current.date(from: DateComponents(year: 2000, month: 1, day: 1))!
    }

    private enum CodingKeys: String, CodingKey {
        case host, port, useHTTPS, testMode, testHost, testPort, backfillDays, backfillMonths, backfillYears
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        host = try c.decode(String.self, forKey: .host)
        port = try c.decode(UInt16.self, forKey: .port)
        useHTTPS = try c.decodeIfPresent(Bool.self, forKey: .useHTTPS) ?? true
        testMode = try c.decodeIfPresent(Bool.self, forKey: .testMode) ?? false
        testHost = try c.decodeIfPresent(String.self, forKey: .testHost) ?? ""
        testPort = try c.decodeIfPresent(UInt16.self, forKey: .testPort) ?? 443

        // Migrate: prefer backfillDays, fall back to backfillMonths, then backfillYears.
        // A stored nil (all data) decodes as absent under every key and stays nil.
        if let days = try c.decodeIfPresent(Int.self, forKey: .backfillDays) {
            backfillDays = days
        } else if let months = try c.decodeIfPresent(Int.self, forKey: .backfillMonths) {
            backfillDays = months * 365 / 12
        } else if let years = try c.decodeIfPresent(Int.self, forKey: .backfillYears) {
            backfillDays = years * 365
        } else {
            backfillDays = nil
        }
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(host, forKey: .host)
        try c.encode(port, forKey: .port)
        try c.encode(useHTTPS, forKey: .useHTTPS)
        try c.encode(testMode, forKey: .testMode)
        try c.encode(testHost, forKey: .testHost)
        try c.encode(testPort, forKey: .testPort)
        try c.encode(backfillDays, forKey: .backfillDays)
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
    }
}
