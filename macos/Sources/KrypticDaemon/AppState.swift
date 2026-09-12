import AppKit
import SwiftUI

@MainActor
final class AppState: ObservableObject {
    @Published var status = SocketClient.DaemonStatus()
    @Published var loginCode: String?
    @Published var loginInProgress = false
    @Published var loginError: String?
    @Published var spawnError: String?
    @Published var updateTitle = "Check for Updates"
    @Published var scanInProgress = false
    @Published var displayAPI = ConfigStore.displayAPI

    let controller = DaemonController()
    private var pollTimer: Timer?

    var binaryAvailable: Bool { DaemonController.binaryURL() != nil }

    func start() {
        ConfigStore.ensureDebugInstallDefault()
        DiagnosticsLog.event("app.start", "version=\(AppVersion.display)")
        refresh()
        pollTimer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh() }
        }
        guard let binary = DaemonController.binaryURL() else { return }
        DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + 8) { [weak self] in
            let result = KrypticProcess.run(binary, ["update", "--check"])
            Task { @MainActor in
                if result.status == 2 {
                    self?.updateTitle = "Update Available…"
                }
            }
        }
    }

    func refresh() {
        Task.detached {
            let status = SocketClient.status()
            await MainActor.run { [weak self] in
                self?.status = status
                if status.running { self?.spawnError = nil }
            }
        }
        // If our child died (crash, logout restart race), bring it back.
        controller.ensureDaemonRunning { [weak self] error in
            self?.spawnError = error
        }
    }

    var menuConnection: SocketClient.Connection {
        if loginInProgress || (binaryAvailable && !status.running) {
            return .connecting
        }
        return status.connection
    }

    var connectionLabel: String {
        switch menuConnection {
        case .connecting:
            if !status.running, spawnError != nil {
                return "Daemon failed to start"
            }
            return "Connecting…"
        case .awaitingApproval:
            return "Awaiting approval"
        case .connected:
            return "Connected"
        case .signedOut:
            return "Signed out"
        }
    }

    func login(addAccount: Bool = false) {
        guard !loginInProgress else { return }
        var api: String?
        if addAccount {
            guard let chosen = ServerURLPresenter.requestForNewAccount() else { return }
            api = chosen
        }
        loginInProgress = true
        loginCode = nil
        loginError = nil
        DiagnosticsLog.event(addAccount ? "auth.login.add" : "auth.login.start")
        controller.login(addAccount: addAccount, api: api) { [weak self] code in
            self?.loginCode = code
        } onFinished: { [weak self] error in
            self?.loginInProgress = false
            self?.loginCode = nil
            self?.loginError = error
            if error == nil {
                DiagnosticsLog.event("auth.login.ok")
            } else {
                DiagnosticsLog.event("auth.login.error")
            }
            self?.refresh()
        }
    }

    func switchProfile(_ id: String) {
        Task.detached {
            _ = SocketClient.switchProfile(id)
            await MainActor.run { [weak self] in
                self?.refresh()
            }
        }
    }

    func deleteProfile(_ id: String) {
        let confirm = NSAlert()
        confirm.messageText = "Remove this account?"
        confirm.informativeText = "This removes the account from this install and revokes its device on that server. Other profiles are not touched."
        confirm.alertStyle = .warning
        confirm.addButton(withTitle: "Remove")
        confirm.addButton(withTitle: "Cancel")
        NSApplication.shared.activate(ignoringOtherApps: true)
        guard confirm.runModal() == .alertFirstButtonReturn else { return }

        Task.detached {
            _ = SocketClient.deleteProfile(id)
            await MainActor.run { [weak self] in
                self?.refresh()
            }
        }
    }

    func cancelLogin() {
        DiagnosticsLog.event("auth.login.cancel")
        controller.cancelLogin()
        loginInProgress = false
        loginCode = nil
    }

    func logout() {
        let confirm = NSAlert()
        confirm.messageText = "Sign out of Kryptic?"
        confirm.informativeText = "Signing out ends this account's session and drops secrets from memory. "
            + "Other saved accounts stay signed in. This machine's keys stay, so the next sign-in "
            + "does not need a new admin grant unless durable device trust is off."
        confirm.alertStyle = .warning
        confirm.addButton(withTitle: "Sign Out")
        confirm.addButton(withTitle: "Cancel")
        NSApplication.shared.activate(ignoringOtherApps: true)
        guard confirm.runModal() == .alertFirstButtonReturn else { return }

        DiagnosticsLog.event("auth.logout")
        controller.logout { [weak self] in
            self?.refresh()
        }
    }

    func refreshSecretsCache() {
        Task.detached {
            _ = SocketClient.flushSecretsCache()
        }
    }

    func scanFolder() {
        guard let binary = DaemonController.binaryURL() else { return }
        ScanPresenter.start(binary: binary) { [weak self] busy in
            self?.scanInProgress = busy
        }
    }

    func checkForUpdates() {
        guard let binary = DaemonController.binaryURL() else { return }
        UpdatePresenter.check(binary: binary, currentVersion: AppVersion.display)
        updateTitle = "Check for Updates"
    }

    func changeServerURL() {
        let current = status.apiUrl ?? displayAPI
        guard let next = ServerURLPresenter.request(current: current) else { return }
        Task.detached {
            _ = SocketClient.setAPI(next)
            await MainActor.run { [weak self] in
                self?.displayAPI = next
                self?.refresh()
            }
        }
    }

    func shutdown() {
        DiagnosticsLog.event("app.stop")
        pollTimer?.invalidate()
        controller.stopDaemonIfOwned()
    }
}
