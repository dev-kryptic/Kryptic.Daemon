import AppKit
import SwiftUI

@MainActor
enum ManageWindowPresenter {
    private static var window: NSWindow?

    static func show(appState: AppState) {
        let content = NSHostingView(rootView: ManageWindow(appState: appState))
        content.frame = NSRect(x: 0, y: 0, width: 380, height: 640)

        if window == nil {
            let panel = NSWindow(
                contentRect: content.frame,
                styleMask: [.titled, .closable],
                backing: .buffered,
                defer: false
            )
            panel.title = "Open Kryptic"
            panel.isReleasedWhenClosed = false
            panel.center()
            window = panel
        }

        window?.contentView = content
        NSApplication.shared.activate(ignoringOtherApps: true)
        window?.makeKeyAndOrderFront(nil)
    }
}
