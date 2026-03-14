import SwiftUI

struct AcknowledgementsView: View {
    var body: some View {
        List {
            Section("Based On") {
                VStack(alignment: .leading, spacing: 8) {
                    Text("HealthBeat")
                        .font(.headline)
                    Text("by kempu")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                    Text("The FreeReps companion app is based on HealthBeat, an open-source iOS app for syncing Apple Health data. HealthBeat was adapted into the FreeReps companion app for the self-hosted FreeReps server.")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                    Link("github.com/kempu/HealthBeat", destination: URL(string: "https://github.com/kempu/HealthBeat")!)
                        .font(.subheadline)
                    Text("Licensed under the MIT License")
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                }
                .padding(.vertical, 4)
            }
        }
        .navigationTitle("Acknowledgements")
        .navigationBarTitleDisplayMode(.inline)
    }
}
