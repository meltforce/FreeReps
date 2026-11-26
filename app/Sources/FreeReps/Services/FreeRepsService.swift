import Foundation

/// Errors from FreeReps HTTP communication.
enum FreeRepsError: LocalizedError {
    case invalidURL
    case httpError(statusCode: Int, body: String)
    case unauthorized
    case forbidden
    case decodingError(String)
    case connectionFailed(String)

    var errorDescription: String? {
        switch self {
        case .invalidURL: return "Invalid FreeReps URL"
        case .httpError(let code, let body): return "HTTP \(code): \(body)"
        case .unauthorized: return "Access token missing or invalid — create one under Settings → Account in the FreeReps web UI"
        case .forbidden: return "Access denied — the access token lacks the api scope or the account is disabled"
        case .decodingError(let msg): return "Decoding error: \(msg)"
        case .connectionFailed(let msg): return "Connection failed: \(msg)"
        }
    }
}

/// Ingest result returned by FreeReps after processing a payload.
struct IngestResult: Codable {
    var metrics_received: Int?
    var metrics_inserted: Int?
    /// Rows that did not exist before; `metrics_inserted` counts changed rows too.
    var metrics_added: Int?
    var metrics_skipped: Int?
    var metrics_rejected: Int?
    var sleep_sessions_inserted: Int?
    var sleep_stages_inserted: Int?
    var workouts_received: Int?
    var workouts_inserted: Int?
    var ecg_recordings_inserted: Int?
    var audiograms_inserted: Int?
    var activity_summaries_inserted: Int?
    var medications_inserted: Int?
    var vision_prescriptions_inserted: Int?
    var state_of_mind_inserted: Int?
    var category_samples_inserted: Int?
    var message: String?

    var totalInserted: Int {
        let a: Int = (metrics_inserted ?? 0) + (sleep_sessions_inserted ?? 0) + (sleep_stages_inserted ?? 0)
        let b: Int = (workouts_inserted ?? 0) + (ecg_recordings_inserted ?? 0) + (audiograms_inserted ?? 0)
        let c: Int = (activity_summaries_inserted ?? 0) + (medications_inserted ?? 0)
        let d: Int = (vision_prescriptions_inserted ?? 0) + (state_of_mind_inserted ?? 0) + (category_samples_inserted ?? 0)
        return a + b + c + d
    }
}

/// Where this client's stored data ends on the server, from `GET /api/v1/sync-state`.
struct ServerSyncState: Decodable {
    struct Checkpoint: Decodable {
        let domain: String
        let item: String?
        let newest_sample: Date
        /// Stored rows; absent on a server without row counts or while it counts.
        let row_count: Int?
    }

    let client: String
    let server_now: Date
    let checkpoints: [Checkpoint]

    /// The newest stored sample of one domain, and for `metrics` of one metric name.
    func newestSample(domain: String, item: String = "") -> Date? {
        checkpoints.first { $0.domain == domain && ($0.item ?? "") == item }?.newest_sample
    }
}

/// Result from the unified import endpoint (CSV uploads).
struct ImportResult: Codable {
    var sets_received: Int
    var sets_inserted: Int64
    var message: String?
}

/// Lightweight HTTP client for FreeReps ingest API.
actor FreeRepsService {

    /// Identifies this app to `/api/v1/ingest`, which Health Auto Export posts to as
    /// well. The server records calls carrying it as `freereps_ios` in import_logs,
    /// and `/api/v1/sync-state` answers for the client it names.
    static let clientHeader = "X-FreeReps-Client"
    static let clientID = "freereps-ios"

    private let session: URLSession
    private let baseURL: URL
    /// Personal access token, nil when none applies to the configured host.
    private let accessToken: String?

    init(config: FreeRepsConfig) {
        self.baseURL = config.baseURL
        self.accessToken = config.accessToken
        let sessionConfig = URLSessionConfiguration.default
        sessionConfig.timeoutIntervalForRequest = 120
        sessionConfig.timeoutIntervalForResource = 300
        self.session = URLSession(configuration: sessionConfig)
    }

    /// POST a FreeReps payload to FreeReps and return the ingest result.
    func ingest(_ payload: FreeRepsPayload) async throws -> IngestResult {
        let url = baseURL.appendingPathComponent("api/v1/ingest/")
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue(Self.clientID, forHTTPHeaderField: Self.clientHeader)
        request.httpBody = try JSONEncoder().encode(payload)

        let data = try await performRequest(request)

        do {
            return try JSONDecoder().decode(IngestResult.self, from: data)
        } catch {
            throw FreeRepsError.decodingError(error.localizedDescription)
        }
    }

    private struct AllowlistEntry: Decodable {
        let metric_name: String
        let enabled: Bool
    }

    /// Metric names the server rejects for the signed-in user — disabled in the
    /// Ingest settings tab or server-wide. The sync skips them before reading HealthKit.
    func fetchDisabledMetrics() async throws -> Set<String> {
        let data = try await get(path: "api/v1/allowlist")
        do {
            let entries = try JSONDecoder().decode([AllowlistEntry].self, from: data)
            return Set(entries.filter { !$0.enabled }.map(\.metric_name))
        } catch {
            throw FreeRepsError.decodingError(error.localizedDescription)
        }
    }

    /// Where this client's data stored on the server ends. The server resolves the
    /// client from the header `get` sends, so the answer covers the app's own
    /// deliveries and not Health Auto Export's.
    func fetchSyncState() async throws -> ServerSyncState {
        let data = try await get(path: "api/v1/sync-state")
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let raw = try decoder.singleValueContainer().decode(String.self)
            // Go writes RFC 3339 with a fractional part of varying length, or none.
            let fractional = ISO8601DateFormatter()
            fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            if let date = fractional.date(from: raw) ?? ISO8601DateFormatter().date(from: raw) {
                return date
            }
            throw DecodingError.dataCorrupted(.init(codingPath: decoder.codingPath, debugDescription: "not an RFC 3339 date: \(raw)"))
        }
        do {
            return try decoder.decode(ServerSyncState.self, from: data)
        } catch {
            throw FreeRepsError.decodingError(error.localizedDescription)
        }
    }

    /// Upload a CSV file to the unified import endpoint.
    func uploadCSV(data: Data) async throws -> ImportResult {
        let url = baseURL.appendingPathComponent("api/v1/import")
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("text/csv", forHTTPHeaderField: "Content-Type")
        request.httpBody = data

        let responseData = try await performRequest(request)

        do {
            return try JSONDecoder().decode(ImportResult.self, from: responseData)
        } catch {
            throw FreeRepsError.decodingError(error.localizedDescription)
        }
    }

    /// Ping FreeReps to verify connectivity and identity.
    func ping() async throws -> String {
        let url = baseURL.appendingPathComponent("api/v1/me")
        var request = URLRequest(url: url)
        request.httpMethod = "GET"

        let data = try await performRequest(request)

        // Return raw JSON for display
        return String(data: data, encoding: .utf8) ?? "{}"
    }

    /// GET a JSON response from a FreeReps endpoint.
    func get(path: String, queryItems: [URLQueryItem] = []) async throws -> Data {
        var components = URLComponents(url: baseURL.appendingPathComponent(path), resolvingAgainstBaseURL: false)!
        if !queryItems.isEmpty {
            components.queryItems = queryItems
        }
        guard let url = components.url else {
            throw FreeRepsError.invalidURL
        }
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue(Self.clientID, forHTTPHeaderField: Self.clientHeader)

        return try await performRequest(request)
    }

    /// Every request passes through here: adds the access token, returns the body of a 200.
    private func performRequest(_ request: URLRequest) async throws -> Data {
        var request = request
        if let accessToken {
            request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        }
        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            throw FreeRepsError.connectionFailed(error.localizedDescription)
        }
        guard let http = response as? HTTPURLResponse else {
            throw FreeRepsError.connectionFailed("Invalid response")
        }
        switch http.statusCode {
        case 200: return data
        case 401: throw FreeRepsError.unauthorized
        case 403: throw FreeRepsError.forbidden
        default:
            throw FreeRepsError.httpError(statusCode: http.statusCode, body: String(data: data, encoding: .utf8) ?? "")
        }
    }
}
