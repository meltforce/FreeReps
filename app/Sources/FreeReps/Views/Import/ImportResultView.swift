import SwiftUI

struct ImportResultView: View {
    @ObservedObject var state: ImportState
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        VStack(spacing: 20) {
            Spacer()

            switch state.status {
            case .uploading:
                ProgressView()
                    .scaleEffect(1.5)
                Text("Uploading...")
                    .font(.headline)
                    .foregroundStyle(.secondary)

            case .success(let setsInserted):
                Image(systemName: "checkmark.circle.fill")
                    .font(.system(size: 48))
                    .foregroundStyle(.green)
                Text("\(setsInserted) sets imported")
                    .font(.headline)

            case .error(let message):
                Image(systemName: "exclamationmark.triangle.fill")
                    .font(.system(size: 48))
                    .foregroundStyle(.red)
                Text(message)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal)

            case .idle:
                EmptyView()
            }

            Spacer()

            if state.status != .uploading {
                Button("Done") {
                    dismiss()
                }
                .buttonStyle(.borderedProminent)
                .padding(.bottom, 32)
            }
        }
        .presentationDetents([.medium])
    }
}
