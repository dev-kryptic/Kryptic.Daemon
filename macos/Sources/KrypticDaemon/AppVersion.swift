import Foundation

enum AppVersion {
    /// Marketing version of this app bundle, stamped by the release pipeline.
    static var display: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "unknown"
    }
}
