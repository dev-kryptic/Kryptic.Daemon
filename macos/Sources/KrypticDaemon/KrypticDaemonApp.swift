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
            MenuBarFalcon()
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
}

private struct MenuBarFalcon: View {
    @Environment(\.colorScheme) private var colorScheme

    var body: some View {
        if let icon = MenuBarIcon.image(darkAppearance: colorScheme == .dark) {
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
                appState.login()
            }
            .disabled(!appState.binaryAvailable)
        }
    }

    @ViewBuilder
    private var statusSection: some View {
        Text("API: \(appState.status.apiUrl ?? appState.displayAPI)")
        if !appState.binaryAvailable {
            Text("kryptic binary not found")
        } else if !appState.status.running {
            if let error = appState.spawnError {
                Text("Daemon: failed to start")
                Text(error)
            } else {
                Text("Daemon: starting…")
            }
        } else if appState.status.authenticated {
            Text("Daemon: online - \(appState.status.email ?? "signed in")")
            if !appState.status.orgKeyGranted {
                Text("Waiting for organization-key grant")
            }
            if let organization = appState.status.organization {
                Text(organization)
            }
        } else {
            Text("Daemon: online - not signed in")
        }
    }
}
