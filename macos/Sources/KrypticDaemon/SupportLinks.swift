import AppKit
import Foundation

enum SupportLinks {
    static let gitHub = URL(string: "https://github.com/dev-kryptic")!
    static let docs = URL(string: "https://docs.kryptic.dev")!

    static func openGitHub() {
        NSWorkspace.shared.open(gitHub)
    }

    static func openDocs() {
        NSWorkspace.shared.open(docs)
    }
}
