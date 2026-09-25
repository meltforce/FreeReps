import SwiftUI

@main
struct FreeRepsApp: App {

    @StateObject private var importState = ImportState()

    init() {
        Self.pinInterfaceLanguageOnce()
    }

    /// The bundle carries a German localization only for the Siri phrases of the
    /// Shortcuts action (AppShortcuts.xcstrings); the interface itself exists in
    /// English alone. Without this, a German device shows the English texts mixed
    /// with German system strings. The app-level AppleLanguages key is what
    /// Settings › FreeReps › Language writes, so it is set once and the user can
    /// still change it there. iOS reads it at launch: it applies from the second one.
    private static func pinInterfaceLanguageOnce() {
        let flag = "interfaceLanguagePinned_v1"
        guard !UserDefaults.standard.bool(forKey: flag) else { return }
        UserDefaults.standard.set(["en"], forKey: "AppleLanguages")
        UserDefaults.standard.set(true, forKey: flag)
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(importState)
                .onOpenURL { url in
                    if url.scheme == SyncLink.scheme {
                        handleSyncLink(url)
                    } else {
                        handleIncomingFile(url)
                    }
                }
                .sheet(isPresented: $importState.showResult) {
                    ImportResultView(state: importState)
                }
        }
    }

    /// `freereps://sync`, opened by tapping the widget.
    private func handleSyncLink(_ url: URL) {
        guard url.host == SyncLink.syncHost else { return }
        let vm = SyncViewModel.shared
        if !vm.isAnySyncRunning { vm.startFullSync() }
    }

    private func handleIncomingFile(_ url: URL) {
        // startAccessingSecurityScopedResource returns false for non-scoped URLs
        // (e.g. inbox copies from share sheet) — proceed with read regardless.
        let scoped = url.startAccessingSecurityScopedResource()
        defer { if scoped { url.stopAccessingSecurityScopedResource() } }

        guard let data = try? Data(contentsOf: url) else {
            importState.status = .error("Failed to read file")
            importState.showResult = true
            return
        }
        performImport(data: data)
    }

    func performImport(data: Data) {
        importState.status = .uploading
        importState.showResult = true

        Task {
            let config = FreeRepsConfig.load()
            let service = FreeRepsService(config: config)
            do {
                let result = try await service.uploadCSV(data: data)
                importState.status = .success(setsInserted: result.sets_inserted)
            } catch {
                importState.status = .error(error.localizedDescription)
            }
        }
    }
}
