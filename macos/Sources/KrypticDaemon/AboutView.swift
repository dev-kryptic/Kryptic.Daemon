import AppKit
import SwiftUI

struct AboutView: View {
    var version: String?

    private var versionLine: String {
        if let version, !version.isEmpty {
            return "Version \(version)"
        }
        return "Version unknown"
    }

    var body: some View {
        VStack(spacing: 20) {
            if let logoURL = AppResources.url(forResource: "logo", withExtension: "png"),
               let logo = NSImage(contentsOf: logoURL) {
                Image(nsImage: logo)
                    .resizable()
                    .scaledToFit()
                    .frame(maxWidth: 220, maxHeight: 80)
            }

            VStack(spacing: 6) {
                Text("Kryptic")
                    .font(.system(size: 22, weight: .semibold, design: .rounded))

                Text("Zero-friction secrets management")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)

                Text(versionLine)
                    .font(.caption)
                    .foregroundStyle(.tertiary)
            }

            Text(
                "Authenticate once. Every project on this machine works. No prefix commands, no .env files."
            )
            .font(.footnote)
            .multilineTextAlignment(.center)
            .foregroundStyle(.secondary)
            .frame(maxWidth: 320)

            VStack(alignment: .leading, spacing: 6) {
                Text("Status")
                    .font(.caption)
                    .foregroundStyle(.tertiary)
                legendRow(ManageTheme.green, "Connected")
                legendRow(ManageTheme.amber, "Connecting or awaiting approval")
                legendRow(ManageTheme.gray, "Signed out")
            }
            .padding(.top, 4)

            Link("kryptic.dev", destination: URL(string: "https://kryptic.dev")!)
                .font(.footnote)
        }
        .padding(28)
        .frame(width: 380)
    }

    private func legendRow(_ color: Color, _ label: String) -> some View {
        HStack(spacing: 8) {
            Circle()
                .fill(color)
                .frame(width: 8, height: 8)
            Text(label)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }
}
