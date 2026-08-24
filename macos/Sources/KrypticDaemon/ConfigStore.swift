import Foundation

/// Same document the Go daemon reads: `~/Library/Application Support/kryptic/config.json`.
enum ConfigStore {
    struct File: Codable {
        var api: String?
    }

    static var directory: URL {
        if let override = ProcessInfo.processInfo.environment["KRYPTIC_CONFIG_DIR"],
           !override.isEmpty {
            return URL(fileURLWithPath: override, isDirectory: true)
        }
        let appSupport = FileManager.default.urls(
            for: .applicationSupportDirectory,
            in: .userDomainMask
        ).first!
        return appSupport.appendingPathComponent("kryptic", isDirectory: true)
    }

    static var fileURL: URL {
        directory.appendingPathComponent("config.json")
    }

    static func load() -> File {
        guard let data = try? Data(contentsOf: fileURL) else { return File() }
        return (try? JSONDecoder().decode(File.self, from: data)) ?? File()
    }

    static func save(_ file: File) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        var payload: [String: String] = [:]
        if let api = file.api, !api.isEmpty {
            payload["api"] = api
        }
        let data = try JSONSerialization.data(withJSONObject: payload, options: [.prettyPrinted, .sortedKeys])
        try data.write(to: fileURL, options: .atomic)
    }

    static var savedAPI: String? {
        let raw = load().api?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return raw.isEmpty ? nil : raw.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
    }

    static func setAPI(_ url: String) throws {
        var file = load()
        file.api = url
        try save(file)
    }

    static func resetAPI() throws {
        var file = load()
        file.api = nil
        try save(file)
    }

    /// The URL the menu should show. Matches Go: env, then file, then default
    /// (debug builds default to the local Daemon BFF).
    static var displayAPI: String {
        if let explicit = ProcessInfo.processInfo.environment["KRYPTIC_API"],
           !explicit.isEmpty {
            return explicit
        }
        if let saved = savedAPI {
            return saved
        }
        #if DEBUG
        return "http://localhost:5237"
        #else
        return "https://daemon.kryptic.dev"
        #endif
    }

    static var envOverrides: Bool {
        if let explicit = ProcessInfo.processInfo.environment["KRYPTIC_API"] {
            return !explicit.isEmpty
        }
        return false
    }
}
