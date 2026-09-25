import SwiftUI

struct ContentView: View {
    @ObservedObject private var syncViewModel = SyncViewModel.shared
    @EnvironmentObject var importState: ImportState

    var body: some View {
        TabView {
            SyncDashboardView(vm: syncViewModel)
                .tabItem {
                    Label("Sync", systemImage: "arrow.triangle.2.circlepath")
                }

            SettingsView(syncViewModel: syncViewModel)
                .tabItem {
                    Label("Settings", systemImage: "gear")
                }
        }
        .tint(Color("Brand"))
        .environmentObject(syncViewModel)
    }
}
