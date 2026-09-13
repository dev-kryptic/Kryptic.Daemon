import AppKit

@MainActor
enum ServerURLPresenter {
    /// Edit the active profile's Daemon BFF. Changing it signs that profile out only.
    static func request(current: String) -> String? {
        prompt(
            title: "Server URI",
            message: "The Daemon BFF for this profile. Cloud and self-host can live side by side. Changing this URL signs this profile out only.",
            current: current,
            confirmChange: true
        )
    }

    /// Ask which server a new account should use, before the browser sign-in.
    static func requestForNewAccount() -> String? {
        prompt(
            title: "New account server",
            message: "Which Daemon BFF should this account use? Use the hosted URL for cloud, or your company's self-hosted URI.",
            current: ConfigStore.displayAPI,
            confirmChange: false
        )
    }

    private static func prompt(title: String, message: String, current: String, confirmChange: Bool) -> String? {
        if ConfigStore.envOverrides {
            alert("KRYPTIC_API is set in the environment and overrides every profile's URL.")
            return nil
        }

        let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 340, height: 24))
        field.stringValue = current
        field.placeholderString = "https://daemon.kryptic.dev"
        field.isEditable = true
        field.isSelectable = true
        field.isBezeled = true
        field.bezelStyle = .roundedBezel
        field.drawsBackground = true
        field.backgroundColor = .textBackgroundColor
        field.textColor = .textColor
        field.usesSingleLineMode = true
        field.cell?.wraps = false
        field.cell?.isScrollable = true

        let prompt = NSAlert()
        prompt.messageText = title
        prompt.informativeText = message
        prompt.alertStyle = .informational
        prompt.accessoryView = field
        prompt.addButton(withTitle: "Kryptic Cloud")
        prompt.addButton(withTitle: "Save")
        prompt.addButton(withTitle: "Cancel")
        prompt.layout()
        prompt.window.initialFirstResponder = field
        NSApplication.shared.activate(ignoringOtherApps: true)
        prompt.window.makeKeyAndOrderFront(nil)
        prompt.window.makeFirstResponder(field)
        let response = prompt.runModal()

        let next: String
        switch response {
        case .alertFirstButtonReturn:
            next = HostLabel.cloudAPI
        case .alertSecondButtonReturn:
            guard let normalized = normalized(field.stringValue) else {
                alert("Server URL must be http or https.")
                return nil
            }
            next = normalized
        default:
            return nil
        }

        if confirmChange, next == current {
            return nil
        }
        if !confirmChange {
            return next
        }

        let confirm = NSAlert()
        confirm.messageText = "Change this profile's server?"
        confirm.informativeText = "This signs this profile out of the previous server. Other profiles keep their URL and session."
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
