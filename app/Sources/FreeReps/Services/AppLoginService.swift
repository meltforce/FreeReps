import AuthenticationServices
import CryptoKit
import Foundation
import UIKit

enum AppLoginError: LocalizedError {
    case cancelled
    /// `GET /auth/methods` answered 404.
    case noLogin
    case noWindow
    case stateMismatch
    case refused(String)
    case invalidCallback
    case network(String)
    case server(statusCode: Int, message: String)
    case keychain

    var errorDescription: String? {
        switch self {
        case .cancelled: return "Sign-in cancelled."
        case .noLogin: return "This server has no login; no sign-in needed."
        case .noWindow: return "No window to present the sign-in sheet."
        case .stateMismatch: return "Sign-in failed: the server's answer does not belong to this sign-in attempt (state mismatch)."
        case .refused(let code): return "Sign-in refused by the server: \(code)"
        case .invalidCallback: return "Sign-in failed: the server's answer carries no code."
        case .network(let msg): return "Connection failed: \(msg)"
        case .server(let code, let msg): return msg.isEmpty ? "Sign-in failed: HTTP \(code)" : "Sign-in failed: \(msg) (HTTP \(code))"
        case .keychain: return "The access token could not be saved to the Keychain."
        }
    }
}

/// Login and display name of the account a sign-in token belongs to.
/// Absent for a manually entered token.
struct AppLoginIdentity: Equatable {
    var login: String
    var displayName: String

    var label: String { displayName.isEmpty ? login : displayName }

    private static let loginKey = "appLogin_login"
    private static let displayNameKey = "appLogin_displayName"

    static func load() -> AppLoginIdentity? {
        guard let login = UserDefaults.standard.string(forKey: loginKey) else { return nil }
        return AppLoginIdentity(login: login, displayName: UserDefaults.standard.string(forKey: displayNameKey) ?? "")
    }

    func save() {
        UserDefaults.standard.set(login, forKey: Self.loginKey)
        UserDefaults.standard.set(displayName, forKey: Self.displayNameKey)
    }

    static func clear() {
        UserDefaults.standard.removeObject(forKey: loginKey)
        UserDefaults.standard.removeObject(forKey: displayNameKey)
    }
}

/// Sign-in through the server's web login: authorization code with PKCE,
/// exchanged for a personal access token.
@MainActor
enum AppLogin {
    static let callbackScheme = "freereps"

    struct Result {
        let token: String
        let identity: AppLoginIdentity
    }

    private struct TokenResponse: Decodable {
        let token: String
        let login: String?
        let display_name: String?
    }

    private struct ErrorResponse: Decodable {
        let error: String
    }

    private static let urlSession = URLSession(configuration: .ephemeral)

    static func signIn(baseURL: URL) async throws -> Result {
        try await probe(baseURL: baseURL)

        let verifier = randomBase64URL(byteCount: 32)
        let challenge = base64URL(Data(SHA256.hash(data: Data(verifier.utf8))))
        let state = randomBase64URL(byteCount: 16)

        var components = URLComponents(url: baseURL.appendingPathComponent("app-login/authorize"), resolvingAgainstBaseURL: false)
        components?.queryItems = [
            URLQueryItem(name: "code_challenge", value: challenge),
            URLQueryItem(name: "code_challenge_method", value: "S256"),
            URLQueryItem(name: "state", value: state),
            URLQueryItem(name: "device_name", value: UIDevice.current.name),
        ]
        // URLComponents leaves "+" unescaped, which a form decoder reads as a space.
        let query = components?.percentEncodedQuery?.replacingOccurrences(of: "+", with: "%2B")
        components?.percentEncodedQuery = query
        guard let authorizeURL = components?.url else { throw AppLoginError.network("Invalid FreeReps URL") }

        let callback = try await WebAuthSession().run(url: authorizeURL)

        let items = URLComponents(url: callback, resolvingAgainstBaseURL: false)?.queryItems ?? []
        func value(_ name: String) -> String? { items.first { $0.name == name }?.value }
        guard value("state") == state else { throw AppLoginError.stateMismatch }
        if let error = value("error") { throw AppLoginError.refused(error) }
        guard let code = value("code"), !code.isEmpty else { throw AppLoginError.invalidCallback }

        return try await exchange(baseURL: baseURL, code: code, verifier: verifier)
    }

    /// Revokes the token on the server. Throws when the server did not confirm it.
    static func revoke(baseURL: URL, token: String) async throws {
        var request = URLRequest(url: baseURL.appendingPathComponent("api/v1/app-login/token"))
        request.httpMethod = "DELETE"
        request.timeoutInterval = 15
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        let (data, http) = try await send(request)
        guard http.statusCode == 204 else { throw serverError(http, data) }
    }

    /// 200 from `/auth/methods` means the server has application login, 404 that it has none.
    private static func probe(baseURL: URL) async throws {
        var request = URLRequest(url: baseURL.appendingPathComponent("auth/methods"))
        request.timeoutInterval = 15
        let (data, http) = try await send(request)
        switch http.statusCode {
        case 200: return
        case 404: throw AppLoginError.noLogin
        default: throw serverError(http, data)
        }
    }

    private static func exchange(baseURL: URL, code: String, verifier: String) async throws -> Result {
        var request = URLRequest(url: baseURL.appendingPathComponent("api/v1/app-login/token"))
        request.httpMethod = "POST"
        request.timeoutInterval = 30
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(["code": code, "code_verifier": verifier])
        let (data, http) = try await send(request)
        guard http.statusCode == 200 else { throw serverError(http, data) }
        guard let body = try? JSONDecoder().decode(TokenResponse.self, from: data), !body.token.isEmpty else {
            throw AppLoginError.server(statusCode: 200, message: "the answer carries no token")
        }
        return Result(token: body.token, identity: AppLoginIdentity(login: body.login ?? "", displayName: body.display_name ?? ""))
    }

    private static func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        do {
            let (data, response) = try await urlSession.data(for: request)
            guard let http = response as? HTTPURLResponse else { throw AppLoginError.network("Invalid response") }
            return (data, http)
        } catch let error as AppLoginError {
            throw error
        } catch {
            throw AppLoginError.network(error.localizedDescription)
        }
    }

    private static func serverError(_ http: HTTPURLResponse, _ data: Data) -> AppLoginError {
        let message = (try? JSONDecoder().decode(ErrorResponse.self, from: data))?.error ?? ""
        return .server(statusCode: http.statusCode, message: message)
    }

    private static func randomBase64URL(byteCount: Int) -> String {
        var generator = SystemRandomNumberGenerator()
        return base64URL(Data((0..<byteCount).map { _ in UInt8.random(in: .min ... .max, using: &generator) }))
    }

    private static func base64URL(_ data: Data) -> String {
        data.base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}

/// One `ASWebAuthenticationSession` run. Shares Safari's cookies, so a session
/// at the identity provider is reused.
@MainActor
private final class WebAuthSession: NSObject, ASWebAuthenticationPresentationContextProviding {
    private var anchor: ASPresentationAnchor?
    private var session: ASWebAuthenticationSession?

    func run(url: URL) async throws -> URL {
        let scenes = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }
        let scene = scenes.first { $0.activationState == .foregroundActive } ?? scenes.first
        guard let window = scene?.keyWindow ?? scene?.windows.first else { throw AppLoginError.noWindow }
        anchor = window

        return try await withCheckedThrowingContinuation { continuation in
            let session = ASWebAuthenticationSession(url: url, callback: .customScheme(AppLogin.callbackScheme)) { callbackURL, error in
                if let callbackURL {
                    continuation.resume(returning: callbackURL)
                } else if let error = error as? ASWebAuthenticationSessionError, error.code == .canceledLogin {
                    continuation.resume(throwing: AppLoginError.cancelled)
                } else {
                    continuation.resume(throwing: AppLoginError.network(error?.localizedDescription ?? "The sign-in sheet closed without an answer."))
                }
            }
            session.presentationContextProvider = self
            session.prefersEphemeralWebBrowserSession = false
            self.session = session
            if !session.start() {
                self.session = nil
                continuation.resume(throwing: AppLoginError.network("The sign-in sheet could not be opened."))
            }
        }
    }

    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        anchor!
    }
}
