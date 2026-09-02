import AppKit

/// Offline folder scan from the menu bar. Shells out to the bundled
/// `kryptic scan` (same embedded gitleaks engine). Nothing is uploaded.
@MainActor
enum ScanPresenter {
    private static var running = false

    static func start(binary: URL, onBusy: @escaping (Bool) -> Void) {
        guard !running else { return }

        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = false
        panel.canCreateDirectories = false
        panel.prompt = "Scan"
        panel.message = "Choose a folder to scan for leaked secrets. The scan runs fully offline on this Mac."
        NSApplication.shared.activate(ignoringOtherApps: true)
        guard panel.runModal() == .OK, let folder = panel.url else { return }

        running = true
        onBusy(true)

        let progress = ScanProgressWindow()
        let session = ScanSession()
        let process = Process()
        process.executableURL = binary
        process.arguments = ["scan", folder.path, "--export", folder.path, "--progress"]
        process.environment = DaemonController.sanitizedEnvironment(api: DaemonController.apiOverride)

        let errPipe = Pipe()
        process.standardOutput = FileHandle.nullDevice
        process.standardError = errPipe

        progress.onCancel = {
            session.cancelled = true
            if process.isRunning {
                process.terminate()
            }
        }
        progress.show()

        errPipe.fileHandleForReading.readabilityHandler = { handle in
            let data = handle.availableData
            Task { @MainActor in
                progress.consumeStderr(data)
            }
        }

        process.terminationHandler = { finished in
            errPipe.fileHandleForReading.readabilityHandler = nil
            let errorData = (try? errPipe.fileHandleForReading.readToEnd()) ?? Data()
            let errorText = String(data: errorData, encoding: .utf8)?
                .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            Task { @MainActor in
                progress.closeQuietly()
                running = false
                onBusy(false)
                if session.cancelled {
                    return
                }
                let status = finished.terminationStatus
                if status == 0 || status == 1 {
                    showResult(folder: folder)
                    return
                }
                let detail = errorText.isEmpty ? "Scan failed." : errorText
                alert(detail)
            }
        }

        do {
            try process.run()
        } catch {
            progress.closeQuietly()
            running = false
            onBusy(false)
            alert(error.localizedDescription)
        }
    }

    private static func showResult(folder: URL) {
        let report = folder.appendingPathComponent("kryptic-scan-report.md")
        guard FileManager.default.fileExists(atPath: report.path),
              let markdown = try? String(contentsOf: report, encoding: .utf8)
        else {
            alert("Scan finished but the report was not written.")
            return
        }
        let files = tableValue(markdown, field: "Files scanned")
        let findings = tableValue(markdown, field: "Findings")
        let notice = NSAlert()
        notice.messageText = findings == 0 ? "No secrets found" : "Potential secrets found"
        notice.informativeText = """
        \(files) files scanned.
        \(findings) potential secret(s) found.

        Report:
        \(report.path)

        This scan ran fully offline. Nothing left this machine.
        """
        notice.alertStyle = .informational
        notice.addButton(withTitle: "Open Report")
        notice.addButton(withTitle: "Close")
        NSApplication.shared.activate(ignoringOtherApps: true)
        if notice.runModal() == .alertFirstButtonReturn {
            NSWorkspace.shared.open(report)
        }
    }

    private static func tableValue(_ markdown: String, field: String) -> Int {
        let needle = "| **\(field)** |"
        guard let range = markdown.range(of: needle) else { return 0 }
        let rest = markdown[range.upperBound...]
        let trimmed = rest.trimmingCharacters(in: .whitespaces)
        var digits = ""
        for character in trimmed {
            if character.isNumber {
                digits.append(character)
            } else if !digits.isEmpty {
                break
            }
        }
        return Int(digits) ?? 0
    }

    private static func alert(_ message: String) {
        let notice = NSAlert()
        notice.messageText = "Kryptic"
        notice.informativeText = message
        notice.alertStyle = .informational
        notice.addButton(withTitle: "OK")
        NSApplication.shared.activate(ignoringOtherApps: true)
        notice.runModal()
    }
}

final class ScanSession: @unchecked Sendable {
    var cancelled = false
}

@MainActor
final class ScanProgressWindow: NSObject, NSWindowDelegate {
    var onCancel: (() -> Void)?

    private var panel: NSPanel?
    private var bar: NSProgressIndicator?
    private var status: NSTextField?
    private var percentLabel: NSTextField?
    private var ignoreClose = false
    private var buffer = ""

    func show() {
        let panel = NSPanel(
            contentRect: NSRect(x: 0, y: 0, width: 380, height: 150),
            styleMask: [.titled, .closable],
            backing: .buffered,
            defer: false
        )
        panel.title = "Scanning"
        panel.isReleasedWhenClosed = false
        panel.level = .floating

        let content = NSView(frame: NSRect(x: 0, y: 0, width: 380, height: 150))
        let status = NSTextField(labelWithString: "Discovering files…")
        status.frame = NSRect(x: 20, y: 104, width: 340, height: 22)
        status.alignment = .center
        status.lineBreakMode = .byTruncatingMiddle
        content.addSubview(status)

        let bar = NSProgressIndicator(frame: NSRect(x: 20, y: 76, width: 340, height: 16))
        bar.minValue = 0
        bar.maxValue = 100
        bar.doubleValue = 0
        bar.isIndeterminate = false
        content.addSubview(bar)

        let percent = NSTextField(labelWithString: "0%")
        percent.frame = NSRect(x: 20, y: 50, width: 340, height: 18)
        percent.alignment = .center
        content.addSubview(percent)

        let cancel = NSButton(title: "Cancel", target: self, action: #selector(cancelClicked))
        cancel.bezelStyle = .rounded
        cancel.frame = NSRect(x: 140, y: 16, width: 100, height: 24)
        content.addSubview(cancel)

        panel.contentView = content
        panel.delegate = self
        panel.center()
        NSApplication.shared.activate(ignoringOtherApps: true)
        panel.makeKeyAndOrderFront(nil)

        self.panel = panel
        self.bar = bar
        self.status = status
        self.percentLabel = percent
    }

    func consumeStderr(_ data: Data) {
        let chunk = String(data: data, encoding: .utf8) ?? ""
        if chunk.isEmpty { return }
        buffer += chunk
        while let newline = buffer.firstIndex(of: "\n") {
            let line = String(buffer[..<newline])
            buffer = String(buffer[buffer.index(after: newline)...])
            apply(line)
        }
    }

    private func apply(_ line: String) {
        let parts = line.split(separator: "\t", maxSplits: 1, omittingEmptySubsequences: false)
        guard let first = parts.first, let percent = Int(first) else { return }
        set(percent: percent, message: parts.count > 1 ? String(parts[1]) : "")
    }

    func set(percent: Int, message: String) {
        let clamped = min(100, max(0, percent))
        bar?.doubleValue = Double(clamped)
        percentLabel?.stringValue = "\(clamped)%"
        if !message.isEmpty {
            status?.stringValue = message
        }
    }

    func closeQuietly() {
        ignoreClose = true
        onCancel = nil
        panel?.delegate = nil
        panel?.close()
        panel = nil
    }

    func windowShouldClose(_ sender: NSWindow) -> Bool {
        if !ignoreClose {
            onCancel?()
        }
        return true
    }

    @objc private func cancelClicked() {
        onCancel?()
        closeQuietly()
    }
}
