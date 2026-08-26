package brand

import _ "embed"

// LogoPNG is the green no-text app icon (black falcon on the brand squircle).
// Used by About, dialogs, and the Windows window/taskbar icon.
//
//go:embed assets/logo.png
var LogoPNG []byte

// LogoICO is the same mark as a multi-size ICO for Windows chrome.
//
//go:embed assets/logo.ico
var LogoICO []byte
