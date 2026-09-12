import Foundation

enum HostLabel {
    static let cloudAPI = "https://daemon.kryptic.dev"
    static let cloudName = "Kryptic Cloud"

    static func display(_ api: String) -> String {
        if isCloud(api) {
            return cloudName
        }
        return api.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    static func isCloud(_ api: String) -> Bool {
        normalize(api).caseInsensitiveCompare(normalize(cloudAPI)) == .orderedSame
    }

    private static func normalize(_ raw: String) -> String {
        raw.trimmingCharacters(in: .whitespacesAndNewlines)
            .trimmingCharacters(in: CharacterSet(charactersIn: "/"))
    }
}
