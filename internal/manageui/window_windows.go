//go:build windows

package manageui

import (
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/dev-kryptic/daemon/internal/winui"
	"golang.org/x/sys/windows"
)

const (
	winWidth     = 380
	viewHeight   = 640
	logoSize     = 40
	maxProfiles  = 8
	btnH         = 32
	profileH     = 46
	trashW       = 34
	pad          = 16
	gap          = 6
	scrollGutter = 18
	scrollStep   = 48

	idAccount = 101
	idAdd     = 102
	idFlush   = 103
	idScan    = 104
	idUpdate  = 105
	idServer  = 106
	idGitHub  = 107
	idDocs    = 108
	idLogs    = 109
	idAbout   = 110
	idQuit    = 111
	idProfile = 200
	idDelete  = 300
)

// Status-dot colors as COLORREF (0x00BBGGRR), same as the tray dot.
const (
	dotGreen = 0x0058D130 // #30d158
	dotAmber = 0x000A9FFF // #ff9f0a
	dotGray  = 0x00938E8E // #8e8e93
)

type form struct {
	theme      winui.Theme
	bgBrush    windows.Handle
	lineBrush  windows.Handle
	titleFont  windows.Handle
	bodyFont   windows.Handle
	smallFont  windows.Handle
	sectionFnt windows.Handle
	buttonFnt  windows.Handle
	logoBMP    windows.Handle

	logoHWND    windows.Handle
	titleHWND   windows.Handle
	versionHWND windows.Handle
	dotHWND     windows.Handle
	statusHWND  windows.Handle
	emailHWND   windows.Handle
	orgHWND     windows.Handle
	apiHWND     windows.Handle
	bannerHWND  windows.Handle
	accountLine windows.Handle
	accountSec  windows.Handle
	accountHWND windows.Handle
	addHWND     windows.Handle
	opsLine     windows.Handle
	opsSec      windows.Handle
	flushHWND   windows.Handle
	scanHWND    windows.Handle
	setLine     windows.Handle
	setSec      windows.Handle
	updateHWND  windows.Handle
	serverHWND  windows.Handle
	helpLine    windows.Handle
	helpSec     windows.Handle
	githubHWND  windows.Handle
	docsHWND    windows.Handle
	logsHWND    windows.Handle
	aboutHWND   windows.Handle
	quitHWND    windows.Handle
	profiles    [maxProfiles]windows.Handle
	deletes     [maxProfiles]windows.Handle
	profileIDs  [maxProfiles]string

	connKind string
	scrollY  int32
	contentH int32
	viewH    int32
	shown    int
}

var (
	className = windows.StringToUTF16Ptr("KrypticManage")
	wndCB     = windows.NewCallback(manageWndProc)

	formMu sync.Mutex
	active *form
	hwnd   windows.Handle
	hnd    Handlers
)

func Show(handlers Handlers) {
	formMu.Lock()
	hnd = handlers
	existing := hwnd
	formMu.Unlock()
	if existing != 0 {
		alive, _, _ := winui.ProcIsWindow.Call(uintptr(existing))
		if alive != 0 {
			winui.ProcShowWindow.Call(uintptr(existing), winui.SWRestore)
			winui.ProcSetForegroundWindow.Call(uintptr(existing))
			return
		}
	}
	beginSession()
	go func() {
		runtime.LockOSThread()
		defer endSession()
		runWindow(handlers)
	}()
}

func runWindow(handlers Handlers) {
	f := &form{theme: winui.CurrentTheme(), viewH: viewHeight}
	f.bgBrush = winui.NewBrush(f.theme.Bg)
	f.lineBrush = winui.NewBrush(f.theme.Border)
	f.titleFont = winui.Font(16, 600, false)
	f.bodyFont = winui.Font(13, 600, false)
	f.smallFont = winui.Font(12, 400, false)
	f.sectionFnt = winui.Font(11, 600, false)
	f.buttonFnt = winui.Font(13, 600, false)
	winui.SetAuxFonts(f.smallFont, winui.FontFace("Segoe MDL2 Assets", 15, 400, false))
	if bmp, err := winui.LogoBitmap(logoSize, f.theme.Bg); err == nil {
		f.logoBMP = bmp
	}

	formMu.Lock()
	active = f
	hnd = handlers
	formMu.Unlock()
	defer func() {
		formMu.Lock()
		active = nil
		hwnd = 0
		formMu.Unlock()
	}()

	instance := winui.Instance()
	small, big := winui.AppIcons()
	class := winui.WndClassEx{
		Size:      uint32(unsafe.Sizeof(winui.WndClassEx{})),
		WndProc:   wndCB,
		Instance:  instance,
		Cursor:    winui.ArrowCursor(),
		Icon:      big,
		IconSm:    small,
		ClassName: className,
	}
	winui.ProcRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))

	style := uintptr(winui.WSCaption | winui.WSSysMenu | winui.WSVScroll | winui.WSClipChildren)
	x, y, winW, winH := winui.CenteredFrame(winWidth, viewHeight, style)
	title, _ := windows.UTF16PtrFromString("Open Kryptic")
	created, _, _ := winui.ProcCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		style|winui.WSVisible,
		uintptr(x), uintptr(y), uintptr(winW), uintptr(winH),
		0, 0, uintptr(instance), 0,
	)
	if created == 0 {
		return
	}
	winui.ApplyChrome(windows.Handle(created), f.theme.Dark)
	formMu.Lock()
	hwnd = windows.Handle(created)
	formMu.Unlock()

	createBody(windows.Handle(created), instance, f)
	paint(f)
	winui.ProcSetTimer.Call(created, 1, 1000, 0)
	winui.ProcShowWindow.Call(created, winui.SWShow)
	winui.ProcUpdateWindow.Call(created)
	winui.ProcSetForegroundWindow.Call(created)
	winui.RunModal()
	winui.ProcKillTimer.Call(created, 1)
}

func createBody(parent, instance windows.Handle, f *form) {
	if f.logoBMP != 0 {
		f.logoHWND = winui.CreateControl(0, "STATIC", "", winui.WSChild|winui.WSVisible|winui.SSBitmap, pad, pad, logoSize, logoSize, parent, instance, 0)
		winui.ProcSendMessageW.Call(uintptr(f.logoHWND), winui.WMSetImage, winui.ImageBitmap, uintptr(f.logoBMP))
	}
	f.titleHWND = label(parent, instance, f.titleFont, "Kryptic", pad, pad, 240, 20)
	f.versionHWND = label(parent, instance, f.smallFont, "", pad, pad, 240, 16)
	f.dotHWND = label(parent, instance, f.bodyFont, "●", pad, pad, 16, 20)
	f.statusHWND = label(parent, instance, f.bodyFont, "", pad, pad, 240, 20)
	f.emailHWND = label(parent, instance, f.bodyFont, "", pad, pad, 240, 18)
	f.orgHWND = label(parent, instance, f.smallFont, "", pad, pad, 240, 16)
	f.apiHWND = label(parent, instance, f.smallFont, "", pad, pad, 240, 16)
	f.bannerHWND = label(parent, instance, f.smallFont, "", pad, pad, 240, 20)

	inner := int32(winWidth - 2*pad - scrollGutter)
	f.accountLine = rule(parent, instance)
	f.accountSec = label(parent, instance, f.sectionFnt, "ACCOUNT", pad, pad, inner, 16)
	f.accountHWND = winui.CreateButton(parent, instance, idAccount, pad, pad, inner, btnH, "Sign In", winui.ButtonPrimary, f.buttonFnt)
	f.addHWND = winui.CreateButton(parent, instance, idAdd, pad, pad, inner, btnH, "Add Account", winui.ButtonGhost, f.buttonFnt)
	rowW := inner - gap - trashW
	for i := 0; i < maxProfiles; i++ {
		f.profiles[i] = winui.CreateButton(parent, instance, uintptr(idProfile+i), pad, pad, rowW, profileH, "", winui.ButtonProfile, f.buttonFnt)
		f.deletes[i] = winui.CreateButton(parent, instance, uintptr(idDelete+i), pad, pad, trashW, trashW, "", winui.ButtonTrash, f.buttonFnt)
		winui.Show(f.profiles[i], false)
		winui.Show(f.deletes[i], false)
	}

	f.opsLine = rule(parent, instance)
	f.opsSec = label(parent, instance, f.sectionFnt, "OPERATIONS", pad, pad, inner, 16)
	f.flushHWND = winui.CreateButton(parent, instance, idFlush, pad, pad, inner, btnH, "Refresh Secrets Cache", winui.ButtonGhost, f.buttonFnt)
	f.scanHWND = winui.CreateButton(parent, instance, idScan, pad, pad, inner, btnH, "Scan for secrets", winui.ButtonGhost, f.buttonFnt)

	f.setLine = rule(parent, instance)
	f.setSec = label(parent, instance, f.sectionFnt, "SETTINGS", pad, pad, inner, 16)
	f.updateHWND = winui.CreateButton(parent, instance, idUpdate, pad, pad, inner, btnH, "Check for Updates", winui.ButtonGhost, f.buttonFnt)
	f.serverHWND = winui.CreateButton(parent, instance, idServer, pad, pad, inner, btnH, "Server URI", winui.ButtonGhost, f.buttonFnt)

	third := (inner - 2*gap) / 3
	f.helpLine = rule(parent, instance)
	f.helpSec = label(parent, instance, f.sectionFnt, "HELP", pad, pad, inner, 16)
	f.githubHWND = winui.CreateButton(parent, instance, idGitHub, pad, pad, third, btnH, "GitHub", winui.ButtonGhost, f.buttonFnt)
	f.docsHWND = winui.CreateButton(parent, instance, idDocs, pad, pad, third, btnH, "Docs", winui.ButtonGhost, f.buttonFnt)
	f.logsHWND = winui.CreateButton(parent, instance, idLogs, pad, pad, third, btnH, "Logs", winui.ButtonGhost, f.buttonFnt)
	f.aboutHWND = winui.CreateButton(parent, instance, idAbout, pad, pad, inner, btnH, "About Kryptic", winui.ButtonGhost, f.buttonFnt)
	f.quitHWND = winui.CreateButton(parent, instance, idQuit, pad, pad, inner, btnH, "Quit Kryptic", winui.ButtonDanger, f.buttonFnt)
}

// layout places every control for the current scroll offset and returns the
// full content height. Only real profile rows take vertical space.
func layout(f *form, shown int, scroll int32) int32 {
	inner := int32(winWidth - 2*pad - scrollGutter)
	rowW := inner - gap - trashW
	third := (inner - 2*gap) / 3
	y := int32(pad)

	place := func(h windows.Handle, x, top, w, hh int32) {
		winui.Move(h, x, top-scroll, w, hh)
	}
	section := func(line, lbl windows.Handle, top int32) int32 {
		place(line, pad, top+6, inner, 1)
		place(lbl, pad, top+17, inner, 16)
		return top + 39
	}

	if f.logoHWND != 0 {
		place(f.logoHWND, pad, y, logoSize, logoSize)
	}
	place(f.titleHWND, pad+logoSize+12, y, 240, 20)
	place(f.versionHWND, pad+logoSize+12, y+20, 240, 16)
	y += logoSize + 14

	place(f.dotHWND, pad, y, 16, 20)
	place(f.statusHWND, pad+18, y, inner-18, 20)
	y += 22
	place(f.emailHWND, pad, y, inner, 18)
	y += 18
	place(f.orgHWND, pad, y, inner, 16)
	y += 16
	place(f.apiHWND, pad, y, inner, 16)
	y += 16
	place(f.bannerHWND, pad, y, inner, 20)
	y += 24

	y = section(f.accountLine, f.accountSec, y)
	place(f.accountHWND, pad, y, inner, btnH)
	y += btnH + gap
	place(f.addHWND, pad, y, inner, btnH)
	y += btnH + gap
	for i := 0; i < maxProfiles; i++ {
		if i < shown {
			place(f.profiles[i], pad, y, rowW, profileH)
			place(f.deletes[i], pad+rowW+gap, y+(profileH-trashW)/2, trashW, trashW)
			winui.Show(f.profiles[i], true)
			winui.Show(f.deletes[i], true)
			y += profileH + gap
		} else {
			winui.Show(f.profiles[i], false)
			winui.Show(f.deletes[i], false)
		}
	}

	y = section(f.opsLine, f.opsSec, y)
	place(f.flushHWND, pad, y, inner, btnH)
	y += btnH + gap
	place(f.scanHWND, pad, y, inner, btnH)
	y += btnH + gap

	y = section(f.setLine, f.setSec, y)
	place(f.updateHWND, pad, y, inner, btnH)
	y += btnH + gap
	place(f.serverHWND, pad, y, inner, btnH)
	y += btnH + gap

	y = section(f.helpLine, f.helpSec, y)
	place(f.githubHWND, pad, y, third, btnH)
	place(f.docsHWND, pad+third+gap, y, third, btnH)
	place(f.logsHWND, pad+2*(third+gap), y, third, btnH)
	y += btnH + gap
	place(f.aboutHWND, pad, y, inner, btnH)
	y += btnH + gap
	place(f.quitHWND, pad, y, inner, btnH)
	y += btnH + pad
	return y
}

func applyScroll(window windows.Handle, f *form, pos int32) {
	max := f.contentH - f.viewH
	if max < 0 {
		max = 0
	}
	if pos < 0 {
		pos = 0
	}
	if pos > max {
		pos = max
	}
	f.scrollY = pos
	f.contentH = layout(f, f.shown, f.scrollY)
	winui.SetVertScroll(window, f.scrollY, f.viewH, f.contentH)
	winui.ProcInvalidateRect.Call(uintptr(window), 0, 1)
}

func label(parent, instance, font windows.Handle, text string, x, y, w, h int32) windows.Handle {
	hwnd := winui.CreateControl(0, "STATIC", text, winui.WSChild|winui.WSVisible|winui.SSEditCtrl, x, y, w, h, parent, instance, 0)
	winui.ProcSendMessageW.Call(uintptr(hwnd), winui.WMSetFont, uintptr(font), 1)
	return hwnd
}

// rule is the 1px separator above each section header, like the macOS window.
func rule(parent, instance windows.Handle) windows.Handle {
	return winui.CreateControl(0, "STATIC", "", winui.WSChild|winui.WSVisible, pad, pad, winWidth-2*pad-scrollGutter, 1, parent, instance, 0)
}

func profileText(p Profile) string {
	name := p.Email
	if name == "" {
		name = p.ID
	}
	var parts []string
	if p.Organization != "" {
		parts = append(parts, p.Organization)
	}
	if host := HostLabel(p.API); host != "" {
		parts = append(parts, host)
	}
	if !p.SignedIn {
		parts = append(parts, "signed out")
	}
	if len(parts) == 0 {
		return name
	}
	return name + "\n" + strings.Join(parts, " · ")
}

func paint(f *form) {
	formMu.Lock()
	handlers := hnd
	window := hwnd
	formMu.Unlock()
	snap := handlers.snap()
	version := snap.Version
	if version == "" {
		version = DefaultVersion()
	}
	winui.SetText(f.versionHWND, "Version "+version)
	if f.connKind != snap.Connection {
		f.connKind = snap.Connection
		winui.ProcInvalidateRect.Call(uintptr(f.dotHWND), 0, 1)
	}
	winui.SetText(f.statusHWND, snap.ConnectionLabel)
	winui.SetText(f.emailHWND, snap.Email)
	winui.SetText(f.orgHWND, snap.Organization)
	if host := HostLabel(snap.API); host != "" {
		winui.SetText(f.apiHWND, host)
	} else {
		winui.SetText(f.apiHWND, "")
	}
	switch {
	case snap.LoginCode != "":
		winui.SetText(f.bannerHWND, "Confirm code in browser: "+snap.LoginCode)
	case snap.LoginError != "":
		winui.SetText(f.bannerHWND, snap.LoginError)
	default:
		winui.SetText(f.bannerHWND, "")
	}

	switch {
	case snap.LoginInProgress:
		winui.SetText(f.accountHWND, "Cancel Sign-In")
		winui.ProcSetWindowLongPtrW.Call(uintptr(f.accountHWND), winui.GWLPUserData, winui.ButtonGhost)
		winui.Enable(f.accountHWND, true)
	case snap.Authenticated:
		winui.SetText(f.accountHWND, "Sign Out")
		winui.ProcSetWindowLongPtrW.Call(uintptr(f.accountHWND), winui.GWLPUserData, winui.ButtonDanger)
		winui.Enable(f.accountHWND, true)
	default:
		winui.SetText(f.accountHWND, "Sign In")
		winui.ProcSetWindowLongPtrW.Call(uintptr(f.accountHWND), winui.GWLPUserData, winui.ButtonPrimary)
		winui.Enable(f.accountHWND, snap.CanLogin)
	}
	winui.Enable(f.addHWND, snap.CanLogin && !snap.LoginInProgress)
	winui.Enable(f.flushHWND, snap.Running)
	if snap.ScanInProgress {
		winui.SetText(f.scanHWND, "Scanning…")
		winui.Enable(f.scanHWND, false)
	} else {
		winui.SetText(f.scanHWND, "Scan for secrets")
		winui.Enable(f.scanHWND, snap.CanLogin)
	}
	title := snap.UpdateTitle
	if title == "" {
		title = "Check for Updates"
	}
	winui.SetText(f.updateHWND, title)
	winui.Enable(f.updateHWND, snap.CanLogin)
	winui.Enable(f.serverHWND, snap.CanLogin)

	shown := 0
	for _, profile := range snap.Profiles {
		if shown >= maxProfiles {
			break
		}
		winui.SetText(f.profiles[shown], profileText(profile))
		kind := winui.ButtonProfile
		if profile.Active {
			kind = winui.ButtonProfileActive
		}
		winui.ProcSetWindowLongPtrW.Call(uintptr(f.profiles[shown]), winui.GWLPUserData, kind)
		winui.Enable(f.profiles[shown], !profile.Active)
		f.profileIDs[shown] = profile.ID
		shown++
	}
	for i := shown; i < maxProfiles; i++ {
		f.profileIDs[i] = ""
	}

	if shown != f.shown || f.contentH == 0 {
		f.shown = shown
		f.contentH = layout(f, shown, f.scrollY)
		if window != 0 {
			applyScroll(window, f, f.scrollY)
		}
	}
}

func statusColor(kind string) uint32 {
	switch kind {
	case "connected":
		return dotGreen
	case "connecting", "awaiting_approval":
		return dotAmber
	default:
		return dotGray
	}
}

func manageWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	formMu.Lock()
	f := active
	formMu.Unlock()
	if f == nil {
		ret, _, _ := winui.ProcDefWindowProcW.Call(hwnd, message, wparam, lparam)
		return ret
	}
	switch message {
	case winui.WMEraseBkgnd:
		return winui.FillBackground(wparam, hwnd, f.bgBrush)
	case winui.WMDrawItem:
		return winui.HandleDrawItem(lparam, f.theme, f.buttonFnt)
	case winui.WMCtlColorBtn:
		return uintptr(f.bgBrush)
	case winui.WMCtlColorStatic:
		switch windows.Handle(lparam) {
		case f.accountLine, f.opsLine, f.setLine, f.helpLine:
			return uintptr(f.lineBrush)
		case f.dotHWND:
			return winui.PaintStatic(wparam, statusColor(f.connKind), f.bgBrush)
		case f.versionHWND, f.orgHWND, f.apiHWND, f.accountSec, f.opsSec, f.setSec, f.helpSec:
			return winui.PaintStatic(wparam, f.theme.Secondary, f.bgBrush)
		case f.bannerHWND:
			return winui.PaintStatic(wparam, f.theme.Danger, f.bgBrush)
		default:
			return winui.PaintStatic(wparam, f.theme.Primary, f.bgBrush)
		}
	case winui.WMVScroll:
		applyScroll(windows.Handle(hwnd), f, nextScroll(f, wparam))
		return 0
	case winui.WMMouseWheel:
		delta := int16(wparam >> 16)
		applyScroll(windows.Handle(hwnd), f, f.scrollY-int32(delta)/120*scrollStep)
		return 0
	case winui.WMTimer:
		paint(f)
		return 0
	case winui.WMCommand:
		handleClick(f, wparam&0xffff)
		return 0
	case winui.WMClose:
		winui.ProcDestroyWindow.Call(hwnd)
		return 0
	case winui.WMDestroy:
		winui.ProcPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := winui.ProcDefWindowProcW.Call(hwnd, message, wparam, lparam)
	return ret
}

func nextScroll(f *form, wparam uintptr) int32 {
	switch wparam & 0xffff {
	case winui.SBLineUp:
		return f.scrollY - scrollStep
	case winui.SBLineDown:
		return f.scrollY + scrollStep
	case winui.SBPageUp:
		return f.scrollY - f.viewH + scrollStep
	case winui.SBPageDown:
		return f.scrollY + f.viewH - scrollStep
	case winui.SBThumbPosition, winui.SBThumbTrack:
		return int32(wparam >> 16)
	default:
		return f.scrollY
	}
}

func handleClick(f *form, id uintptr) {
	formMu.Lock()
	handlers := hnd
	formMu.Unlock()
	switch id {
	case idAccount:
		switch {
		case handlers.snap().LoginInProgress:
			handlers.fire("cancelLogin", "")
		case handlers.snap().Authenticated:
			handlers.fire("signOut", "")
		default:
			handlers.fire("signIn", "")
		}
	case idAdd:
		handlers.fire("addAccount", "")
	case idFlush:
		handlers.fire("flush", "")
	case idScan:
		handlers.fire("scan", "")
	case idUpdate:
		handlers.fire("update", "")
	case idServer:
		handlers.fire("serverURI", "")
	case idGitHub:
		handlers.fire("github", "")
	case idDocs:
		handlers.fire("docs", "")
	case idLogs:
		handlers.fire("logs", "")
	case idAbout:
		handlers.fire("about", "")
	case idQuit:
		handlers.fire("quit", "")
	default:
		if id >= idDelete && id < idDelete+maxProfiles {
			i := int(id - idDelete)
			if f.profileIDs[i] != "" {
				handlers.fire("deleteProfile", f.profileIDs[i])
			}
			return
		}
		if id >= idProfile && id < idProfile+maxProfiles {
			i := int(id - idProfile)
			if f.profileIDs[i] != "" {
				handlers.fire("switchProfile", f.profileIDs[i])
			}
		}
	}
}
