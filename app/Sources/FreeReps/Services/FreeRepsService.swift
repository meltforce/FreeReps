import Foundation

/// Errors from FreeReps HTTP communication.
enum FreeRepsError: LocalizedError {
    case invalidURL
    case httpError(statusCode: Int, body: String)
    case decodingError(String)
    case connectionFailed(String)

    var errorDescription: String? {
        switch self {
        case .invalidURL: return "Invalid FreeReps URL"
        case .httpError(let code, let body): return "HTTP \(code): \(body)"
        case .decodingError(let msg): return "Decoding error: \(msg)"
        case .connectionFailed(let msg): return "Connection failed: \(msg)"
        }
    }
}

/// Ingest result returned by FreeReps after processing a payload.
struct IngestResult: Codable {
    var metrics_received: Int?
    var metrics_inserted: Int?
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

/// Result from the unified import endpoint (CSV uploads).
struct ImportResult: Codable {
    var sets_received: Int
    var sets_inserted: Int64
    var message: String?
}

/// Lightweight HTTP client for FreeReps ingest API.
/// Replaces the 748-line MySQLService with ~50 lines of URLSession.
actor FreeRepsService {

    private let session: URLSession
    private let baseURL: URL

    init(config: FreeRepsConfig) {
        self.baseURL = config.baseURL
        let sessionConfig = URLSessionConfiguration.default
        sessionConfig.timeoutIntervalForRequest = 30
        sessionConfig.timeoutIntervalForResource = 300
        // Trust Tailscale certificates
        self.session = URLSession(configuration: sessionConfig)
    }

    /// POST a FreeReps payload to FreeReps and return the ingest result.
    func ingest(_ payload: FreeRepsPayload) async throws -> IngestResult {
        let url = baseURL.appendingPathComponent("api/v1/ingest/")
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(payload)

        let (data, response) = try await performRequest(request)

        guard let http = response as? HTTPURLResponse else {
            throw FreeRepsError.connectionFailed("Invalid response")
        }
        guard http.statusCode == 200 else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw FreeRepsError.httpError(statusCode: http.statusCode, body: body)
        }

        do {
            return try JSONDecoder().decode(IngestResult.self, from: data)
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

        let (responseData, response) = try await performRequest(request)

        guard let http = response as? HTTPURLResponse else {
            throw FreeRepsError.connectionFailed("Invalid response")
        }
        guard http.statusCode == 200 else {
            let body = String(data: responseData, encoding: .utf8) ?? ""
            throw FreeRepsError.httpError(statusCode: http.statusCode, body: body)
        }

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

        let (data, response) = try await performRequest(request)

        guard let http = response as? HTTPURLResponse else {
            throw FreeRepsError.connectionFailed("Invalid response")
        }
        guard http.statusCode == 200 else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw FreeRepsError.httpError(statusCode: http.statusCode, body: body)
        }

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

        let (data, response) = try await performRequest(request)

        guard let http = response as? HTTPURLResponse else {
            throw FreeRepsError.connectionFailed("Invalid response")
        }
        guard http.statusCode == 200 else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw FreeRepsError.httpError(statusCode: http.statusCode, body: body)
        }
        return data
    }

    private func performRequest(_ request: URLRequest) async throws -> (Data, URLResponse) {
        do {
            return try await session.data(for: request)
        } catch {
            throw FreeRepsError.connectionFailed(error.localizedDescription)
        }
    }
}
