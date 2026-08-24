import AppKit

@MainActor
enum UpdatePresenter {
    static func check(binary: URL, currentVersion: String, quietIfCurrent: Bool = false) {
        let result = KrypticProcess.run(binary, ["update", "--check"])
        if result.status == 0 {
            if !quietIfCurrent {
                alert("Kryptic \(currentVersion) is already the latest version.")
            }
            return
        }
        if result.status != 2 {
            let detail = result.stderr.isEmpty ? result.stdout : result.stderr
            alert(detail.isEmpty ? "Could not check for updates." : detail)
            return
        }

        let latest = parseLatest(result.stdout) ?? "a newer version"
        let confirm = NSAlert()
        confirm.messageText = "Update available"
        confirm.informativeText = "Version \(latest) is available (you have \(currentVersion)). Update now?"
        confirm.alertStyle = .informational
        confirm.addButton(withTitle: "Update")
        confirm.addButton(withTitle: "Later")
        NSApplication.shared.activate(ignoringOtherApps: true)
        guard confirm.runModal() == .alertFirstButtonReturn else { return }

        Task.detached {
            let install = KrypticProcess.run(binary, ["update", "--installer"])
            await MainActor.run {
                if install.status != 0 {
                    let detail = install.stderr.isEmpty ? install.stdout : install.stderr
                    alert(detail.isEmpty ? "Update failed." : detail)
                    return
                }
                alert("The installer is open. Finish it to complete the update. Your sign-in is kept.")
            }
        }
    }

    static func parseLatest(_ stdout: String) -> String? {
        // "kryptic 0.0.8 -> 0.0.9 available"
        let parts = stdout.split(separator: " ")
        guard let index = parts.firstIndex(of: "->"), index + 1 < parts.count else {
            return nil
        }
        return String(parts[index + 1])
    }

    private static func alert(_ message: String) {
        let notice = NSAlert()
        notice.messageText = "Kryptic"
        notice.informativeText = message
        notice.alertStyle = .informational
        notice.addButton(withTitle: "OK")
        NSApplication.shared.activate(ignoringOtherApps: true)
        notice.runModal()
    }
}
