// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "FreeReps",
    platforms: [.iOS("16.2")],
    targets: [
        .target(
            name: "FreeReps",
            path: "Sources/FreeReps",
            resources: [
                .process("Resources")
            ]
        )
    ]
)
