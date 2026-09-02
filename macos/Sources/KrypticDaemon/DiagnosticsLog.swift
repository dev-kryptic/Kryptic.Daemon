import AppKit
import Darwin
import Foundation

/// Size-capped diagnostics log shared with the Go daemon/CLI.
/// Function only: no secrets, tokens, emails, or display names.
enum DiagnosticsLog {
    static let fileName = "kryptic.krypticlog"
    static let backupName = "kryptic.krypticlog.1"
    static let maxFileBytes: UInt64 = 2 * 1024 * 1024

    static var directory: URL {
        ConfigStore.directory.appendingPathComponent("logs", isDirectory: true)
    }

    static var fileURL: URL {
        directory.appendingPathComponent(fileName)
    }

    static func event(_ name: String, _ fields: String...) {
        let stamp = ISO8601DateFormatter().string(from: Date())
        var parts = [stamp, "app", sanitize(name)]
        for field in fields {
            parts.append(sanitize(field))
        }
        append(parts.joined(separator: " ") + "\n")
    }

    static func reveal() {
        let fm = FileManager.default
        try? fm.createDirectory(at: directory, withIntermediateDirectories: true)
        if !fm.fileExists(atPath: fileURL.path) {
            event("logs.created")
        }
        NSWorkspace.shared.activateFileViewerSelecting([fileURL])
    }

    private static func append(_ line: String) {
        let fm = FileManager.default
        try? fm.createDirectory(at: directory, withIntermediateDirectories: true)
        let path = fileURL.path
        if let attrs = try? fm.attributesOfItem(atPath: path),
           let size = attrs[.size] as? UInt64,
           size + UInt64(line.utf8.count) > maxFileBytes {
            let backup = directory.appendingPathComponent(backupName)
            try? fm.removeItem(at: backup)
            try? fm.moveItem(at: fileURL, to: backup)
        }

        let fd = open(path, O_WRONLY | O_CREAT | O_APPEND, 0o600)
        guard fd >= 0 else { return }
        defer { close(fd) }
        line.withCString { ptr in
            _ = write(fd, ptr, strlen(ptr))
        }
    }

    static func sanitize(_ raw: String) -> String {
        var text = raw.replacingOccurrences(of: "\n", with: " ")
        text = text.replacingOccurrences(of: "\r", with: " ")
        if let email = try? NSRegularExpression(pattern: #"(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b"#) {
            text = email.stringByReplacingMatches(in: text, range: NSRange(text.startIndex..., in: text), withTemplate: "[email]")
        }
        if let bearer = try? NSRegularExpression(pattern: #"(?i)bearer\s+\S+"#) {
            text = bearer.stringByReplacingMatches(in: text, range: NSRange(text.startIndex..., in: text), withTemplate: "bearer [token]")
        }
        if let opaque = try? NSRegularExpression(pattern: #"\b[A-Za-z0-9_-]{32,}\b"#) {
            text = opaque.stringByReplacingMatches(in: text, range: NSRange(text.startIndex..., in: text), withTemplate: "[redacted]")
        }
        return text.trimmingCharacters(in: .whitespaces)
    }
}
