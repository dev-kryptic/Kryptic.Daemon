import Foundation

/// Supervises the Go `kryptic` binary:
/// spawn `kryptic start` if no daemon answers the socket, keep it alive while the app
/// runs, and shell out to the same binary for `login` / `logout`.
@MainActor
final class DaemonController {
    private(set) var daemonProcess: Process?
    private var loginProcess: Process?

    /// Where the Go daemon and CLI point. An explicit KRYPTIC_API in the app's
    /// environment always wins; debug builds default to the local Daemon BFF as run
    /// from the IDE (port 5237; the docker-compose stack serves it on 5211 instead)
    /// so login opens the local management client.
    /// Release builds leave it unset and the Go binary uses the hosted URL.
    static let apiOverride: String? = {
        if let explicit = ProcessInfo.processInfo.environment["KRYPTIC_API"] {
            return explicit
        }
        #if DEBUG
        return "http://localhost:5237"
        #else
        return nil
        #endif
    }()

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

    /// Starts `kryptic start` unless a daemon (ours or launchd's) already answers.
    func ensureDaemonRunning() {
        guard daemonProcess?.isRunning != true,
              !SocketClient.status().running,
              let binary = Self.binaryURL() else { return }

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

    /// Stops the daemon only if this app spawned it - an externally managed
    /// daemon (launchd, terminal) is left alone.
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
