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
            if let icon = MenuBarIcon.image() {
                Image(nsImage: icon)
                    .renderingMode(.template)
                    .frame(width: 18, height: 18)
            } else {
                Image(systemName: "key.fill")
            }
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

private struct MenuBarContent: View {
    @ObservedObject var appState: AppState

    var body: some View {
        Group {
            statusSection

            Divider()

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

            Button("Refresh Secrets Cache") {
                appState.refreshSecretsCache()
            }
            .disabled(!appState.status.running)

            Button("About Kryptic…") {
                AboutWindowPresenter.show(version: appState.status.daemonVersion)
            }

            Divider()

            Button("Quit Kryptic") {
                appState.shutdown()
                NSApplication.shared.terminate(nil)
            }
        }
    }

    @ViewBuilder
    private var statusSection: some View {
        if let api = DaemonController.apiOverride {
            Text("API: \(api)")
        }
        if !appState.binaryAvailable {
            Text("kryptic binary not found")
        } else if !appState.status.running {
            Text("Daemon: starting…")
        } else if appState.status.authenticated {
            Text("Daemon: online - \(appState.status.email ?? "signed in")")
            if let organization = appState.status.organization {
                Text(organization)
            }
        } else {
            Text("Daemon: online - not signed in")
        }
    }
}
