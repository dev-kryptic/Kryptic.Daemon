import AppKit
import Foundation

let resources = URL(fileURLWithPath: CommandLine.arguments[1], isDirectory: true)
let svgURL = resources.appendingPathComponent("falcon-black.svg")

guard let source = NSImage(contentsOf: svgURL) else {
    fputs("Failed to load falcon-black.svg\n", stderr)
    exit(1)
}

func savePNG(_ image: NSImage, size: NSSize, to url: URL) throws {
    let copy = NSImage(size: size)
    copy.lockFocus()
    NSColor.clear.set()
    NSRect(origin: .zero, size: size).fill()
    source.draw(
        in: NSRect(origin: .zero, size: size),
        from: NSRect(origin: .zero, size: source.size),
        operation: .sourceOver,
        fraction: 1,
        respectFlipped: true,
        hints: nil
    )
    copy.unlockFocus()

    guard let tiff = copy.tiffRepresentation,
          let rep = NSBitmapImageRep(data: tiff),
          let data = rep.representation(using: .png, properties: [:]) else {
        throw NSError(domain: "GenerateIcons", code: 1)
    }
    try data.write(to: url)
}

do {
    try savePNG(source, size: NSSize(width: 256, height: 256), to: resources.appendingPathComponent("AppIcon.png"))
    print("Generated AppIcon.png")
} catch {
    fputs("Failed to write AppIcon.png: \(error)\n", stderr)
    exit(1)
}
