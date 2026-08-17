import AppKit
import SwiftUI

struct AboutView: View {
    @Environment(\.colorScheme) private var colorScheme

    private var logoName: String {
        colorScheme == .dark ? "logo-white" : "logo"
    }

    var body: some View {
        VStack(spacing: 20) {
            if let logoURL = Bundle.module.url(forResource: logoName, withExtension: "png"),
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

                Text("Version 0.1.0 (preview)")
                    .font(.caption)
                    .foregroundStyle(.tertiary)
            }

            Text(
                "Authenticate once. Every project on this machine works - no prefix commands, no .env files."
            )
            .font(.footnote)
            .multilineTextAlignment(.center)
            .foregroundStyle(.secondary)
            .frame(maxWidth: 320)

            Link("kryptic.dev", destination: URL(string: "https://kryptic.dev")!)
                .font(.footnote)
        }
        .padding(28)
        .frame(width: 380)
    }
}
