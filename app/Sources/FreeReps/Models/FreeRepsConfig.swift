import Foundation
import Security

struct FreeRepsConfig: Codable, Equatable {
    var host: String
    var port: UInt16
    var useHTTPS: Bool = true
    /// Max days of HealthKit history to backfill. nil = all data (back to 2000).
    /// Legacy: `backfillMonths` and `backfillYears` are decoded and converted to days.
    var backfillDays: Int? = 730

    init(host: String, port: UInt16, useHTTPS: Bool = true, backfillDays: Int? = 730) {
        self.host = host
        self.port = port
        self.useHTTPS = useHTTPS
        self.backfillDays = backfillDays
    }

    static let `default` = FreeRepsConfig(
        host: "freereps.your-tailnet.ts.net",
        port: 443,
        useHTTPS: true,
        backfillDays: 730
    )

    var baseURL: URL {
        let scheme = useHTTPS ? "https" : "http"
        return URL(string: "\(scheme)://\(host):\(port)")!
    }

    /// Token for the `Authorization` header, nil when none is stored.
    var accessToken: String? {
        let token = AccessTokenStore.load()
        return token.isEmpty ? nil : token
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
        // Until 2.3.0 a "test mode" held a second server that replaced host and
        // port while it was on. A stored config with it on keeps talking to that
        // server: its host and port become the regular ones.
        if try c.decodeIfPresent(Bool.self, forKey: .testMode) ?? false,
           let testHost = try c.decodeIfPresent(String.self, forKey: .testHost), !testHost.isEmpty {
            host = testHost
            port = try c.decodeIfPresent(UInt16.self, forKey: .testPort) ?? 443
        }

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

/// Personal access token of the regular host, in the Keychain.
/// Accessible after first unlock, because the Shortcuts action runs while the device is locked.
enum AccessTokenStore {
    private static let query: [String: Any] = [
        kSecClass as String: kSecClassGenericPassword,
        kSecAttrService as String: "com.meltforce.freereps.access-token",
        kSecAttrAccount as String: "default",
    ]

    static func load() -> String {
        var q = query
        q[kSecReturnData as String] = true
        q[kSecMatchLimit as String] = kSecMatchLimitOne
        var item: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &item) == errSecSuccess,
              let data = item as? Data else { return "" }
        return String(data: data, encoding: .utf8) ?? ""
    }

    /// Stores the trimmed token; an empty one deletes the item.
    @discardableResult
    static func save(_ token: String) -> Bool {
        let trimmed = token.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            let status = SecItemDelete(query as CFDictionary)
            return status == errSecSuccess || status == errSecItemNotFound
        }
        let attributes: [String: Any] = [
            kSecValueData as String: Data(trimmed.utf8),
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlock,
        ]
        var status = SecItemUpdate(query as CFDictionary, attributes as CFDictionary)
        if status == errSecItemNotFound {
            status = SecItemAdd(query.merging(attributes) { $1 } as CFDictionary, nil)
        }
        return status == errSecSuccess
    }
}
