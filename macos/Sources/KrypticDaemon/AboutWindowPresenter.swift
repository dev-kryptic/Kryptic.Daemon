import AppKit
import SwiftUI

@MainActor
enum AboutWindowPresenter {
    private static var window: NSWindow?

    static func show(version: String? = nil) {
        let content = NSHostingView(rootView: AboutView(version: version))
        content.frame = NSRect(x: 0, y: 0, width: 380, height: 360)

        if window == nil {
            let panel = NSPanel(
                contentRect: content.frame,
                styleMask: [.titled, .closable, .fullSizeContentView],
                backing: .buffered,
                defer: false
            )
            panel.title = "About Kryptic"
            panel.titlebarAppearsTransparent = true
            panel.isReleasedWhenClosed = false
            panel.center()
            window = panel
        }

        window?.contentView = content
        NSApplication.shared.activate(ignoringOtherApps: true)
        window?.center()
        window?.makeKeyAndOrderFront(nil)
    }
}
