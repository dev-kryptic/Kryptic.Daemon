import AppKit
import SwiftUI

enum ManageTheme {
    static let bg = Color(nsColor: .windowBackgroundColor)
    static let text = Color(nsColor: .labelColor)
    static let muted = Color(nsColor: .secondaryLabelColor)
    static let faint = Color(nsColor: .tertiaryLabelColor)
    static let surface = Color(nsColor: .controlBackgroundColor)
    static let line = Color(nsColor: .separatorColor)
    static let accent = Color(red: 27 / 255, green: 138 / 255, blue: 90 / 255)
    static let accentDark = Color(red: 48 / 255, green: 209 / 255, blue: 88 / 255)
    static let accentText = Color.white
    static let green = Color(red: 48 / 255, green: 209 / 255, blue: 88 / 255)
    static let amber = Color(red: 255 / 255, green: 159 / 255, blue: 10 / 255)
    static let gray = Color(red: 142 / 255, green: 142 / 255, blue: 147 / 255)
    static let danger = Color(red: 225 / 255, green: 29 / 255, blue: 72 / 255)
}
