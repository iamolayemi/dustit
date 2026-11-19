// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "DustIt",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(name: "DustIt", path: "Sources/DustIt")
    ]
)
