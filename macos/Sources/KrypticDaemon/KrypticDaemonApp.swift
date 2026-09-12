import AppKit
import SwiftUI

@main
struct KrypticDaemonApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var appState: AppState

    init() {
        guard SingleInstanceGuard.acquire() else {
            exit(0)
        }
        NSApplication.shared.setActivationPolicy(.accessory)

        let state = AppState()
        _appState = StateObject(wrappedValue: state)
        AppDelegate.shared = state
        state.start()
    }

    var body: some Scene {
        MenuBarExtra {
            MenuBarContent(appState: appState)
        } label: {
            MenuBarFalcon(connection: appState.menuConnection)
        }
        .menuBarExtraStyle(.menu)
    }
}

/// Terminates the daemon child process when the app quits.
final class AppDelegate: NSObject, NSApplicationDelegate {
    @MainActor static var shared: AppState?

    func applicationWillTerminate(_ notification: Notification) {
        MainActor.assumeIsolated {
            AppDelegate.shared?.shutdown()
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        false
    }
}

private struct MenuBarFalcon: View {
    @Environment(\.colorScheme) private var colorScheme
    let connection: SocketClient.Connection

    var body: some View {
        if let icon = MenuBarIcon.image(darkAppearance: colorScheme == .dark, connection: connection) {
            Image(nsImage: icon)
                .resizable()
                .interpolation(.high)
                .frame(width: 18, height: 18)
        } else {
            Image(systemName: "key.fill")
        }
    }
}

private struct MenuBarContent: View {
    @ObservedObject var appState: AppState

    var body: some View {
        Group {
            statusSection

            Divider()

            signInSection

            Menu("Accounts") {
                ForEach(appState.status.profiles) { profile in
                    Button {
                        appState.switchProfile(profile.id)
                    } label: {
                        Text(profile.active ? "✓ \(profile.title)" : profile.title)
                    }
                    .disabled(profile.active)
                }
                if !appState.status.profiles.isEmpty {
                    Divider()
                    ForEach(appState.status.profiles) { profile in
                        Button("Remove \(profile.email.isEmpty ? profile.id : profile.email)…") {
                            appState.deleteProfile(profile.id)
                        }
                    }
                    Divider()
                }
                Button("Add Account…") {
                    appState.login(addAccount: true)
                }
                .disabled(!appState.binaryAvailable || appState.loginInProgress)
            }

            Button("Open Kryptic") {
                ManageWindowPresenter.show(appState: appState)
            }

            Divider()

            Menu("Operations") {
                Button("Refresh Secrets Cache") {
                    appState.refreshSecretsCache()
                }
                .disabled(!appState.status.running)

                Button(appState.scanInProgress ? "Scanning…" : "Scan for secrets") {
                    appState.scanFolder()
                }
                .disabled(!appState.binaryAvailable || appState.scanInProgress)
            }

            Menu("Settings") {
                Button(appState.updateTitle) {
                    appState.checkForUpdates()
                }
                .disabled(!appState.binaryAvailable)

                Button("Server URI") {
                    appState.changeServerURL()
                }
                .disabled(!appState.binaryAvailable)
            }

            Menu("Help & Support") {
                Button("GitHub") {
                    SupportLinks.openGitHub()
                }
                Button("Documentation") {
                    SupportLinks.openDocs()
                }
                Button("Reveal Diagnostics Log") {
                    DiagnosticsLog.reveal()
                }
            }

            Button("About Kryptic") {
                AboutWindowPresenter.show(version: AppVersion.display)
            }

            Divider()

            Button("Quit Kryptic") {
                appState.shutdown()
                NSApplication.shared.terminate(nil)
            }
            .keyboardShortcut("q")
        }
    }

    @ViewBuilder
    private var signInSection: some View {
        if appState.loginInProgress {
            if let code = appState.loginCode {
                Text("Confirm code in browser: \(code)")
            }
            Button("Cancel Sign-In") {
                appState.cancelLogin()
            }
        } else if appState.status.authenticated {
            Button("Sign Out…") {
                appState.logout()
            }
        } else {
            if let error = appState.loginError {
                Text("⚠️ \(error)")
            }
            Button("Sign In…") {
                appState.login(addAccount: false)
            }
            .disabled(!appState.binaryAvailable)
        }
    }

    @ViewBuilder
    private var statusSection: some View {
        Text(HostLabel.display(appState.status.apiUrl ?? appState.displayAPI))
        if !appState.binaryAvailable {
            Text("kryptic binary not found")
        } else {
            Text(menuStatusLabel)
            if appState.status.authenticated, let organization = appState.status.organization {
                Text(organization)
            }
            if let error = appState.spawnError, !appState.status.running {
                Text(error)
            }
        }
    }

    private var menuStatusLabel: String {
        switch appState.menuConnection {
        case .connecting:
            return "🟠 \(appState.connectionLabel)"
        case .awaitingApproval:
            return "🟠 \(appState.connectionLabel)"
        case .connected:
            if let email = appState.status.email, !email.isEmpty {
                return "🟢 \(appState.connectionLabel) · \(email)"
            }
            return "🟢 \(appState.connectionLabel)"
        case .signedOut:
            return "⚪ \(appState.connectionLabel)"
        }
    }
}
