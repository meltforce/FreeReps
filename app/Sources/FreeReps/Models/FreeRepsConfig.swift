import Foundation

struct FreeRepsConfig: Codable, Equatable {
    var host: String
    var port: UInt16
    var useHTTPS: Bool = true
    var testMode: Bool = false
    var testHost: String = ""
    var testPort: UInt16 = 443
    /// Max months of HealthKit history to backfill. nil = all data (back to 2000).
    /// Legacy: `backfillYears` is decoded and converted to months for backward compatibility.
    var backfillMonths: Int? = 24

    /// Backward-compatible computed property. Setting this updates backfillMonths.
    var backfillYears: Int? {
        get { backfillMonths.map { $0 / 12 } }
        set { backfillMonths = newValue.map { $0 * 12 } }
    }

    init(host: String, port: UInt16, useHTTPS: Bool = true, testMode: Bool = false, testHost: String = "", testPort: UInt16 = 443, backfillMonths: Int? = 24) {
        self.host = host
        self.port = port
        self.useHTTPS = useHTTPS
        self.testMode = testMode
        self.testHost = testHost
        self.testPort = testPort
        self.backfillMonths = backfillMonths
    }

    static let `default` = FreeRepsConfig(
        host: "freereps.your-tailnet.ts.net",
        port: 443,
        useHTTPS: true,
        testMode: false,
        testHost: "",
        testPort: 443,
        backfillMonths: 24
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

    /// Earliest date to backfill from, based on `backfillMonths`.
    var backfillStartDate: Date {
        if let months = backfillMonths {
            return Calendar.current.date(byAdding: .month, value: -months, to: Date()) ?? Date()
        }
        return Calendar.current.date(from: DateComponents(year: 2000, month: 1, day: 1))!
    }

    private enum CodingKeys: String, CodingKey {
        case host, port, useHTTPS, testMode, testHost, testPort, backfillMonths, backfillYears
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        host = try c.decode(String.self, forKey: .host)
        port = try c.decode(UInt16.self, forKey: .port)
        useHTTPS = try c.decodeIfPresent(Bool.self, forKey: .useHTTPS) ?? true
        testMode = try c.decodeIfPresent(Bool.self, forKey: .testMode) ?? false
        testHost = try c.decodeIfPresent(String.self, forKey: .testHost) ?? ""
        testPort = try c.decodeIfPresent(UInt16.self, forKey: .testPort) ?? 443

        // Migrate: prefer backfillMonths, fall back to backfillYears * 12
        if let months = try c.decodeIfPresent(Int.self, forKey: .backfillMonths) {
            backfillMonths = months
        } else if let years = try c.decodeIfPresent(Int.self, forKey: .backfillYears) {
            backfillMonths = years * 12
        } else {
            backfillMonths = nil
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
        try c.encode(backfillMonths, forKey: .backfillMonths)
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
