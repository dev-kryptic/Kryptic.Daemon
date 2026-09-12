import AppKit

enum MenuBarIcon {
    static func image(darkAppearance: Bool, connection: SocketClient.Connection) -> NSImage? {
        let name = darkAppearance ? "falcon" : "falcon-black"
        let base = loadSVG(named: name) ?? loadPNG(named: "MenuBarIcon")
        guard let base else { return nil }

        let size = NSSize(width: 18, height: 18)
        let composed = NSImage(size: size)
        composed.lockFocus()
        base.draw(
            in: NSRect(origin: .zero, size: size),
            from: .zero,
            operation: .sourceOver,
            fraction: 1,
            respectFlipped: true,
            hints: [.interpolation: NSImageInterpolation.high]
        )

        let diameter: CGFloat = 6
        let rect = NSRect(x: size.width - diameter, y: 0, width: diameter, height: diameter)
        (darkAppearance ? NSColor.black : NSColor.white).setFill()
        NSBezierPath(ovalIn: rect.insetBy(dx: -0.7, dy: -0.7)).fill()
        statusColor(connection).setFill()
        NSBezierPath(ovalIn: rect).fill()
        composed.unlockFocus()
        composed.isTemplate = false
        return composed
    }

    private static func statusColor(_ connection: SocketClient.Connection) -> NSColor {
        switch connection {
        case .connected:
            return NSColor(srgbRed: 48 / 255, green: 209 / 255, blue: 88 / 255, alpha: 1)
        case .connecting, .awaitingApproval:
            return NSColor(srgbRed: 255 / 255, green: 159 / 255, blue: 10 / 255, alpha: 1)
        case .signedOut:
            return NSColor(srgbRed: 142 / 255, green: 142 / 255, blue: 147 / 255, alpha: 1)
        }
    }

    private static func loadSVG(named name: String) -> NSImage? {
        guard let url = AppResources.url(forResource: name, withExtension: "svg") else {
            return nil
        }
        return NSImage(contentsOf: url)
    }

    private static func loadPNG(named name: String) -> NSImage? {
        guard let url = AppResources.url(forResource: name, withExtension: "png"),
              let image = NSImage(contentsOf: url) else {
            return nil
        }
        image.size = NSSize(width: 18, height: 18)
        return image
    }
}
