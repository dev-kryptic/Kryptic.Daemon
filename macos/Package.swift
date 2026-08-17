// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "KrypticDaemon",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "KrypticDaemon", targets: ["KrypticDaemon"]),
    ],
    targets: [
        .executableTarget(
            name: "KrypticDaemon",
            path: "Sources/KrypticDaemon",
            exclude: ["Info.plist"],
            resources: [.process("Resources")]
        ),
    ]
)
