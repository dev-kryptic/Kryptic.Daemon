import Foundation

/// Supervises the Go `kryptic` binary:
/// spawn `kryptic start` if no daemon answers the socket, keep it alive while the app
/// runs, and shell out to the same binary for `login` / `logout`.
@MainActor
final class DaemonController {
    private(set) var daemonProcess: Process?
    private var loginProcess: Process?

    /// Where the Go daemon and CLI point. An explicit KRYPTIC_API in the app's
    /// environment always wins; debug builds default to the local Daemon BFF
    /// unless the user saved a URL. Release builds leave it unset so the Go
    /// binary reads config.json or the hosted default.
    static var apiOverride: String? {
        if let explicit = ProcessInfo.processInfo.environment["KRYPTIC_API"],
           !explicit.isEmpty {
            return explicit
        }
        #if DEBUG
        if ConfigStore.savedAPI == nil {
            return "http://localhost:5237"
        }
        #endif
        return nil
    }

    private static func makeProcess(_ binary: URL, _ arguments: [String]) -> Process {
        let process = Process()
        process.executableURL = binary
        process.arguments = arguments
        if let api = apiOverride {
            process.environment = ProcessInfo.processInfo.environment.merging(["KRYPTIC_API": api]) { _, new in new }
        }
        return process
    }

    /// The Go CLI. Packaged apps carry it next to the app executable; during
    /// `swift run` development we fall back to the repo build, then to PATH installs.
    static func binaryURL() -> URL? {
        let candidates = [
            Bundle.main.bundleURL.appendingPathComponent("Contents/MacOS/kryptic"),
            URL(fileURLWithPath: #filePath) // …/daemon/macos/Sources/KrypticDaemon/DaemonController.swift
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .deletingLastPathComponent()
                .appendingPathComponent("kryptic"),
            URL(fileURLWithPath: "/opt/homebrew/bin/kryptic"),
            URL(fileURLWithPath: "/usr/local/bin/kryptic"),
        ]
        return candidates.first { FileManager.default.isExecutableFile(atPath: $0.path) }
    }

    /// Version reported by the bundled CLI (`kryptic version` prints `kryptic x.y.z`).
    private static var cachedBundledVersion: String?

    static func bundledVersion() -> String? {
        if let cached = cachedBundledVersion { return cached }
        guard let binary = binaryURL() else { return nil }
        let process = makeProcess(binary, ["version"])
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = FileHandle.nullDevice
        do {
            try process.run()
            process.waitUntilExit()
        } catch {
            return nil
        }
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        let text = String(data: data, encoding: .utf8)?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let parts = text.split(separator: " ")
        guard let last = parts.last, !last.isEmpty else { return nil }
        let version = String(last)
        cachedBundledVersion = version
        return version
    }

    /// Starts the bundled daemon. If an older install is still answering the
    /// socket, it is stopped first. The Keychain session is not cleared.
    func ensureDaemonRunning() {
        guard let binary = Self.binaryURL() else { return }

        let status = SocketClient.status()
        if status.running {
            let ours = Self.bundledVersion()
            if ours == nil || status.daemonVersion == ours {
                return
            }
            stopAnyDaemon()
        } else if daemonProcess?.isRunning == true {
            return
        }

        let process = Self.makeProcess(binary, ["start"])
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        do {
            try process.run()
            daemonProcess = process
        } catch {
            NSLog("kryptic: failed to start daemon: \(error)")
        }
    }

    /// Stops whichever daemon currently owns the socket (this app's child, a
    /// leftover CLI, or a LaunchAgent). Does not log the user out.
    func stopAnyDaemon() {
        loginProcess?.terminate()
        loginProcess = nil
        if let process = daemonProcess, process.isRunning {
            process.terminate()
            process.waitUntilExit()
        }
        daemonProcess = nil

        guard let binary = Self.binaryURL() else { return }
        let stop = Self.makeProcess(binary, ["stop"])
        stop.standardOutput = FileHandle.nullDevice
        stop.standardError = FileHandle.nullDevice
        try? stop.run()
        stop.waitUntilExit()
    }

    /// Stops the daemon only if this app spawned it - an externally managed
    /// daemon (launchd, terminal) is left alone. Used on Quit so we do not
    /// kill a LaunchAgent the user still wants.
    func stopDaemonIfOwned() {
        loginProcess?.terminate()
        loginProcess = nil
        if let process = daemonProcess, process.isRunning {
            process.terminate()
        }
        daemonProcess = nil
    }

    func restartDaemonIfOwned() {
        guard let process = daemonProcess else { return }
        if process.isRunning {
            process.terminate()
            process.waitUntilExit()
        }
        daemonProcess = nil
        ensureDaemonRunning()
    }

    /// Runs `kryptic login` (the CLI opens the browser and polls the device flow).
    /// `onCode` fires with the user code to confirm; `onFinished` with nil on success
    /// or the CLI's error line on failure, so the menu can say what went wrong.
    func login(onCode: @escaping (String) -> Void, onFinished: @escaping (String?) -> Void) {
        guard loginProcess?.isRunning != true, let binary = Self.binaryURL() else { return }

        let process = Self.makeProcess(binary, ["login"])

        let pipe = Pipe()
        let errorPipe = Pipe()
        process.standardOutput = pipe
        process.standardError = errorPipe
        pipe.fileHandleForReading.readabilityHandler = { handle in
            guard let text = String(data: handle.availableData, encoding: .utf8),
                  let match = text.range(of: #"(?<=browser: )\S+"#, options: .regularExpression)
            else { return }
            let code = String(text[match])
            DispatchQueue.main.async { onCode(code) }
        }
        process.terminationHandler = { finished in
            pipe.fileHandleForReading.readabilityHandler = nil
            let errorData = errorPipe.fileHandleForReading.readAvailableDataSafely()
            let errorText = String(data: errorData, encoding: .utf8)?
                .trimmingCharacters(in: .whitespacesAndNewlines)
            let failed = finished.terminationStatus != 0
            DispatchQueue.main.async {
                onFinished(failed ? (errorText?.isEmpty == false ? errorText : "sign-in failed") : nil)
            }
        }

        do {
            try process.run()
            loginProcess = process
        } catch {
            NSLog("kryptic: failed to run login: \(error)")
            onFinished(error.localizedDescription)
        }
    }

    func cancelLogin() {
        loginProcess?.terminate()
        loginProcess = nil
    }

    /// Runs `kryptic logout`, then restarts an owned daemon so its in-memory
    /// access token (cached up to 15 min) is dropped immediately.
    func logout(onFinished: @escaping () -> Void) {
        guard let binary = Self.binaryURL() else { return }

        let process = Self.makeProcess(binary, ["logout"])
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        process.terminationHandler = { _ in
            DispatchQueue.main.async { [weak self] in
                self?.restartDaemonIfOwned()
                onFinished()
            }
        }
        try? process.run()
    }
}

private extension FileHandle {
    /// availableData throws an ObjC exception on a closed handle; reading the rest
    /// of the pipe after termination via readToEnd is the safe variant.
    func readAvailableDataSafely() -> Data {
        (try? readToEnd()) ?? Data()
    }
}
