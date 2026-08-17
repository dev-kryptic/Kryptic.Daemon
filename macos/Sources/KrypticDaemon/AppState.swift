import SwiftUI

@MainActor
final class AppState: ObservableObject {
    @Published var status = SocketClient.DaemonStatus()
    @Published var loginCode: String?
    @Published var loginInProgress = false
    @Published var loginError: String?

    let controller = DaemonController()
    private var pollTimer: Timer?

    var binaryAvailable: Bool { DaemonController.binaryURL() != nil }

    func start() {
        controller.ensureDaemonRunning()
        refresh()
        pollTimer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh() }
        }
    }

    func refresh() {
        Task.detached {
            let status = SocketClient.status()
            await MainActor.run { [weak self] in self?.status = status }
        }
        // If our child died (crash, logout restart race), bring it back.
        controller.ensureDaemonRunning()
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

    func shutdown() {
        pollTimer?.invalidate()
        controller.stopDaemonIfOwned()
    }
}
