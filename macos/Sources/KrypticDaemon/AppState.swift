import SwiftUI

@MainActor
final class AppState: ObservableObject {
    @Published var status = SocketClient.DaemonStatus()
    @Published var loginCode: String?
    @Published var loginInProgress = false
    @Published var loginError: String?
    @Published var spawnError: String?
    @Published var updateTitle = "Check for Updates…"
    @Published var displayAPI = ConfigStore.displayAPI

    let controller = DaemonController()
    private var pollTimer: Timer?

    var binaryAvailable: Bool { DaemonController.binaryURL() != nil }

    func start() {
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

    func login() {
        guard !loginInProgress else { return }
        loginInProgress = true
        loginCode = nil
        loginError = nil
        controller.login { [weak self] code in
            self?.loginCode = code
        } onFinished: { [weak self] error in
            self?.loginInProgress = false
            self?.loginCode = nil
            self?.loginError = error
            self?.refresh()
        }
    }

    func cancelLogin() {
        controller.cancelLogin()
        loginInProgress = false
        loginCode = nil
    }

    func logout() {
        controller.logout { [weak self] in
            self?.refresh()
        }
    }

    func refreshSecretsCache() {
        Task.detached {
            _ = SocketClient.flushSecretsCache()
        }
    }

    func checkForUpdates() {
        guard let binary = DaemonController.binaryURL() else { return }
        UpdatePresenter.check(binary: binary, currentVersion: AppVersion.display)
        updateTitle = "Check for Updates…"
    }

    func changeServerURL() {
        guard let next = ServerURLPresenter.request() else { return }
        controller.logout { [weak self] in
            do {
                if next == "https://daemon.kryptic.dev" {
                    try ConfigStore.resetAPI()
                } else {
                    try ConfigStore.setAPI(next)
                }
            } catch {
                return
            }
            self?.controller.stopAnyDaemon()
            self?.controller.ensureDaemonRunning { [weak self] error in
                self?.spawnError = error
            }
            self?.displayAPI = ConfigStore.displayAPI
            self?.refresh()
        }
    }

    func shutdown() {
        pollTimer?.invalidate()
        controller.stopDaemonIfOwned()
    }
}
