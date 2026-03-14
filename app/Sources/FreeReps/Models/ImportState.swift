import Foundation

@MainActor
class ImportState: ObservableObject {

    enum Status: Equatable {
        case idle
        case uploading
        case success(setsInserted: Int64)
        case error(String)
    }

    @Published var status: Status = .idle
    @Published var showResult = false
}
