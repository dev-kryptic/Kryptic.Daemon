import AppKit

enum MenuBarIcon {
    static func image(darkAppearance: Bool) -> NSImage? {
        let name = darkAppearance ? "falcon" : "falcon-black"
        guard let image = loadSVG(named: name) else {
            return loadPNG(named: "MenuBarIcon")
        }

        image.size = NSSize(width: 18, height: 18)
        image.isTemplate = false
        return image
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
        image.isTemplate = true
        return image
    }
}
