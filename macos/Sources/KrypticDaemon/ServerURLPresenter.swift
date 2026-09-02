import AppKit

@MainActor
enum ServerURLPresenter {
    /// Returns the URL to save, or nil if the user cancelled.
    static func request() -> String? {
        if ConfigStore.envOverrides {
            alert(
                "KRYPTIC_API is set in the environment and overrides the saved URL."
            )
            return nil
        }

        let field = NSTextField(string: ConfigStore.displayAPI)
        field.placeholderString = "https://daemon.kryptic.dev"
        field.frame = NSRect(x: 0, y: 0, width: 340, height: 24)

        let prompt = NSAlert()
        prompt.messageText = "Server URI"
        prompt.informativeText = "The Daemon BFF this app talks to. Changing it signs you out."
        prompt.alertStyle = .informational
        prompt.accessoryView = field
        prompt.addButton(withTitle: "Save")
        prompt.addButton(withTitle: "Use Default")
        prompt.addButton(withTitle: "Cancel")
        NSApplication.shared.activate(ignoringOtherApps: true)
        let response = prompt.runModal()

        let current = ConfigStore.displayAPI
        let next: String
        switch response {
        case .alertFirstButtonReturn:
            guard let normalized = normalized(field.stringValue) else {
                alert("Server URL must be http or https.")
                return nil
            }
            next = normalized
        case .alertSecondButtonReturn:
            next = "https://daemon.kryptic.dev"
        default:
            return nil
        }

        if next == current {
            return nil
        }

        let confirm = NSAlert()
        confirm.messageText = "Change server?"
        confirm.informativeText = "This signs you out of the current server. You will need to sign in again."
        confirm.alertStyle = .warning
        confirm.addButton(withTitle: "Change Server")
        confirm.addButton(withTitle: "Cancel")
        guard confirm.runModal() == .alertFirstButtonReturn else { return nil }
        return next
    }

    private static func normalized(_ raw: String) -> String? {
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
            .trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        guard let url = URL(string: trimmed),
              let scheme = url.scheme?.lowercased(),
              scheme == "http" || scheme == "https",
              url.host != nil else {
            return nil
        }
        return trimmed
    }

    private static func alert(_ message: String) {
        let notice = NSAlert()
        notice.messageText = "Kryptic"
        notice.informativeText = message
        notice.alertStyle = .informational
        notice.addButton(withTitle: "OK")
        notice.runModal()
    }
}
