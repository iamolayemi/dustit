// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "MacCleanBar",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(name: "MacCleanBar", path: "Sources/MacCleanBar")
    ]
)
