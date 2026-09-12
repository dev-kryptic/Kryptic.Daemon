import AppKit
import SwiftUI

/// Open Kryptic. Layout matches the shared panel spec: 380pt
/// column, status dot, account actions, operations, settings, help.
struct ManageWindow: View {
    @ObservedObject var appState: AppState
    @Environment(\.colorScheme) private var colorScheme

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                header
                statusBlock
                section("Account") { accountActions }
                section("Operations") { operations }
                section("Settings") { settings }
                section("Help") { help }
            }
            .padding(16)
        }
        .frame(width: 380, height: 640)
        .background(ManageTheme.bg)
    }

    private var header: some View {
        HStack(spacing: 12) {
            if let url = AppResources.url(forResource: "AppIcon", withExtension: "png"),
               let image = NSImage(contentsOf: url) {
                Image(nsImage: image)
                    .resizable()
                    .frame(width: 40, height: 40)
                    .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
            }
            VStack(alignment: .leading, spacing: 2) {
                Text("Kryptic")
                    .font(.system(size: 16, weight: .semibold))
                Text("Version \(AppVersion.display)")
                    .font(.system(size: 11))
                    .foregroundStyle(ManageTheme.faint)
            }
            Spacer()
        }
    }

    private var statusBlock: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 8) {
                Circle()
                    .fill(statusColor)
                    .frame(width: 8, height: 8)
                    .shadow(color: statusColor.opacity(0.35), radius: 3)
                Text(appState.connectionLabel)
                    .font(.system(size: 13, weight: .semibold))
            }
            if let email = appState.status.email, !email.isEmpty {
                Text(email).font(.system(size: 13, weight: .medium))
            }
            if let org = appState.status.organization, !org.isEmpty {
                Text(org).font(.system(size: 12)).foregroundStyle(ManageTheme.muted)
            }
            Text(HostLabel.display(appState.status.apiUrl ?? appState.displayAPI))
                .font(.system(size: 12))
                .foregroundStyle(ManageTheme.muted)
            if let code = appState.loginCode {
                banner("Confirm code in browser: \(code)", error: false)
            }
            if let error = appState.loginError {
                banner(error, error: true)
            }
        }
    }

    @ViewBuilder
    private var accountActions: some View {
        if appState.loginInProgress {
            panelButton("Cancel Sign-In", kind: .surface) { appState.cancelLogin() }
        } else if appState.status.authenticated {
            panelButton("Sign Out", kind: .danger) { appState.logout() }
        } else {
            panelButton("Sign In", kind: .primary, disabled: !appState.binaryAvailable) {
                appState.login(addAccount: false)
            }
        }
        panelButton("Add Account", kind: .surface, disabled: !appState.binaryAvailable || appState.loginInProgress) {
            appState.login(addAccount: true)
        }
        ForEach(appState.status.profiles) { profile in
            ProfileRow(profile: profile, accent: accent) {
                appState.switchProfile(profile.id)
            } onDelete: {
                appState.deleteProfile(profile.id)
            }
        }
    }

    @ViewBuilder
    private var operations: some View {
        panelButton("Refresh Secrets Cache", kind: .surface, disabled: !appState.status.running) {
            appState.refreshSecretsCache()
        }
        panelButton(appState.scanInProgress ? "Scanning…" : "Scan for secrets", kind: .surface, disabled: !appState.binaryAvailable || appState.scanInProgress) {
            appState.scanFolder()
        }
    }

    @ViewBuilder
    private var settings: some View {
        panelButton(appState.updateTitle, kind: .surface, disabled: !appState.binaryAvailable) {
            appState.checkForUpdates()
        }
        panelButton("Server URI", kind: .surface, disabled: !appState.binaryAvailable) {
            appState.changeServerURL()
        }
    }

    @ViewBuilder
    private var help: some View {
        HStack(spacing: 8) {
            panelButton("GitHub", kind: .surface) { SupportLinks.openGitHub() }
            panelButton("Docs", kind: .surface) { SupportLinks.openDocs() }
            panelButton("Logs", kind: .surface) { DiagnosticsLog.reveal() }
        }
        panelButton("About Kryptic", kind: .surface) {
            AboutWindowPresenter.show(version: AppVersion.display)
        }
        panelButton("Quit Kryptic", kind: .danger) {
            appState.shutdown()
            NSApplication.shared.terminate(nil)
        }
    }

    private func section<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title.uppercased())
                .font(.system(size: 11, weight: .semibold))
                .tracking(0.4)
                .foregroundStyle(ManageTheme.faint)
            VStack(spacing: 8) { content() }
        }
        .padding(.top, 4)
        .overlay(alignment: .top) {
            Rectangle().fill(ManageTheme.line).frame(height: 1)
        }
        .padding(.top, 10)
    }

    private func banner(_ text: String, error: Bool) -> some View {
        Text(text)
            .font(.system(size: 12))
            .foregroundStyle(error ? ManageTheme.danger : ManageTheme.muted)
            .padding(.horizontal, 10)
            .padding(.vertical, 8)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(ManageTheme.surface)
            .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
    }

    private func panelButton(_ title: String, kind: ButtonKind, disabled: Bool = false, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(.system(size: 13, weight: .semibold))
                .frame(maxWidth: .infinity)
                .padding(.vertical, 8)
                .background(kind.background(accent: accent))
                .foregroundStyle(kind.foreground)
                .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        }
        .buttonStyle(.plain)
        .disabled(disabled)
        .opacity(disabled ? 0.45 : 1)
    }

    private var statusColor: Color {
        switch appState.menuConnection {
        case .connected: return ManageTheme.green
        case .connecting, .awaitingApproval: return ManageTheme.amber
        case .signedOut: return ManageTheme.gray
        }
    }

    private var accent: Color {
        colorScheme == .dark ? ManageTheme.accentDark : ManageTheme.accent
    }

}

private struct ProfileRow: View {
    let profile: SocketClient.Profile
    let accent: Color
    let onSwitch: () -> Void
    let onDelete: () -> Void
    @State private var hovering = false

    var body: some View {
        HStack(spacing: 8) {
            Button(action: onSwitch) {
                HStack(spacing: 8) {
                    Text(profile.active ? "✓" : "")
                        .foregroundStyle(accent)
                        .frame(width: 14)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(profile.email.isEmpty ? profile.id : profile.email)
                        if !profile.organization.isEmpty || !profile.api.isEmpty || !profile.signedIn {
                            Text(profileMeta)
                                .font(.system(size: 11))
                                .foregroundStyle(ManageTheme.muted)
                        }
                    }
                    Spacer()
                }
                .padding(.horizontal, 10)
                .padding(.vertical, 8)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(ManageTheme.surface)
                .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
                .overlay(
                    RoundedRectangle(cornerRadius: 10, style: .continuous)
                        .stroke(profile.active ? accent : Color.clear, lineWidth: 1)
                )
            }
            .buttonStyle(.plain)
            .disabled(profile.active)

            Button(action: onDelete) {
                Image(systemName: "trash")
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(ManageTheme.danger)
                    .frame(width: 28, height: 28)
            }
            .buttonStyle(.plain)
            .opacity(hovering ? 1 : 0)
            .help("Delete this account")
            .accessibilityLabel("Delete \(profile.email.isEmpty ? profile.id : profile.email)")
        }
        .onHover { hovering = $0 }
    }

    private var profileMeta: String {
        var parts: [String] = []
        if !profile.organization.isEmpty {
            parts.append(profile.organization)
        }
        if !profile.api.isEmpty {
            parts.append(HostLabel.display(profile.api))
        }
        if !profile.signedIn {
            parts.append("signed out")
        }
        return parts.joined(separator: " · ")
    }
}

private enum ButtonKind {
    case primary, surface, danger

    func background(accent: Color) -> Color {
        switch self {
        case .primary: return accent
        case .surface: return ManageTheme.surface
        case .danger: return ManageTheme.danger.opacity(0.12)
        }
    }

    var foreground: Color {
        switch self {
        case .primary: return ManageTheme.accentText
        case .surface: return ManageTheme.text
        case .danger: return ManageTheme.danger
        }
    }
}

