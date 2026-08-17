import AppKit
import SwiftUI

@MainActor
enum AboutWindowPresenter {
    private static var window: NSWindow?

    static func show() {
        if window == nil {
            let content = NSHostingView(rootView: AboutView())
            content.frame = NSRect(x: 0, y: 0, width: 380, height: 360)

            let panel = NSPanel(
                contentRect: content.frame,
                styleMask: [.titled, .closable, .fullSizeContentView],
                backing: .buffered,
                defer: false
            )
            panel.title = "About Kryptic"
            panel.titlebarAppearsTransparent = true
            panel.isReleasedWhenClosed = false
            panel.contentView = content
            panel.center()
            window = panel
        }

        NSApplication.shared.activate(ignoringOtherApps: true)
        window?.center()
        window?.makeKeyAndOrderFront(nil)
    }
}
