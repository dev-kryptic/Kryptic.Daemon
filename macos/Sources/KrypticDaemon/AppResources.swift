import Foundation

/// Locates the SwiftPM resource bundle without SwiftPM's generated
/// `Bundle.module`, which only searches the app bundle root. Code signing
/// forbids anything but Contents/ at the root ("unsealed contents"), so the
/// packaged app carries the bundle in Contents/Resources instead. Unlike
/// `Bundle.module`, this returns nil instead of calling fatalError, so a
/// missing bundle degrades to fallback icons rather than a silent crash.
enum AppResources {
    private static let bundleName = "KrypticDaemon_KrypticDaemon.bundle"

    private static let bundle: Bundle? = {
        let candidates = [
            Bundle.main.resourceURL, // packaged app: Kryptic.app/Contents/Resources
            Bundle.main.bundleURL,   // swift run / swift build: next to the executable
        ]
        for base in candidates {
            guard let base else { continue }
            if let found = Bundle(url: base.appendingPathComponent(bundleName)) {
                return found
            }
        }
        return nil
    }()

    static func url(forResource name: String, withExtension ext: String) -> URL? {
        bundle?.url(forResource: name, withExtension: ext)
    }
}
