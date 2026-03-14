import Foundation

// Generic row returned by the data browser REST queries
struct HealthRecord: Identifiable, Decodable {
    let id: String
    let startDate: Date
    let endDate: Date
    let value: Double?
    let valueLabel: String?
    let unit: String?
    let sourceName: String?
    let typeLabel: String?

    enum CodingKeys: String, CodingKey {
        case id
        case startDate = "start_date"
        case endDate = "end_date"
        case value
        case valueLabel = "value_label"
        case unit
        case sourceName = "source_name"
        case typeLabel = "type"
    }
}

struct MedicationRecord: Identifiable, Decodable {
    let id: String
    let medicationName: String?
    let dosage: String?
    let startDate: Date
    let endDate: Date?
    let sourceName: String?

    enum CodingKeys: String, CodingKey {
        case id
        case medicationName = "medication_name"
        case dosage
        case startDate = "start_date"
        case endDate = "end_date"
        case sourceName = "source_name"
    }
}

struct WorkoutRecord: Identifiable, Decodable {
    let id: String
    let activityType: String
    let startDate: Date
    let endDate: Date
    let durationSeconds: Double
    let energyKcal: Double?
    let distanceMeters: Double?
    let sourceName: String?

    var durationFormatted: String {
        let mins = Int(durationSeconds / 60)
        let secs = Int(durationSeconds) % 60
        if mins >= 60 {
            return "\(mins / 60)h \(mins % 60)m"
        }
        return "\(mins)m \(secs)s"
    }

    enum CodingKeys: String, CodingKey {
        case id
        case activityType = "activity_type"
        case startDate = "start_date"
        case endDate = "end_date"
        case durationSeconds = "duration_seconds"
        case energyKcal = "total_energy_burned_kcal"
        case distanceMeters = "total_distance_meters"
        case sourceName = "source_name"
    }
}
