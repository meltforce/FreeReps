import SwiftUI

struct ContentView: View {
    @ObservedObject private var syncViewModel = SyncViewModel.shared
    @EnvironmentObject var importState: ImportState
    @State private var selectedTab = Tab.sync
    @State private var showConnectionSettings = false

    private enum Tab { case sync, settings }

    var body: some View {
        TabView(selection: $selectedTab) {
            SyncDashboardView(vm: syncViewModel, onOpenConnectionSettings: {
                showConnectionSettings = true
                selectedTab = .settings
            })
                .tabItem {
                    Label("Sync", systemImage: "arrow.triangle.2.circlepath")
                }
                .tag(Tab.sync)

            SettingsView(syncViewModel: syncViewModel, showConnectionSettings: $showConnectionSettings)
                .tabItem {
                    Label("Settings", systemImage: "gear")
                }
                .tag(Tab.settings)
        }
        .tint(Color("Brand"))
        .environmentObject(syncViewModel)
    }
}
