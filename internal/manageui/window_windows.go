//go:build windows

package manageui

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/dev-kryptic/daemon/internal/winui"
	"golang.org/x/sys/windows"
)

const (
	winWidth    = 380
	logoSize    = 40
	maxProfiles = 3
	btnH        = 32
	pad         = 16
	gap         = 6

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
)

type form struct {
	theme      winui.Theme
	bgBrush    windows.Handle
	titleFont  windows.Handle
	bodyFont   windows.Handle
	smallFont  windows.Handle
	sectionFnt windows.Handle
	buttonFnt  windows.Handle
	logoBMP    windows.Handle

	titleHWND   windows.Handle
	versionHWND windows.Handle
	statusHWND  windows.Handle
	emailHWND   windows.Handle
	orgHWND     windows.Handle
	apiHWND     windows.Handle
	bannerHWND  windows.Handle
	accountHWND windows.Handle
	addHWND     windows.Handle
	flushHWND   windows.Handle
	scanHWND    windows.Handle
	updateHWND  windows.Handle
	serverHWND  windows.Handle
	githubHWND  windows.Handle
	docsHWND    windows.Handle
	logsHWND    windows.Handle
	aboutHWND   windows.Handle
	quitHWND    windows.Handle
	profiles    [maxProfiles]windows.Handle
	profileIDs  [maxProfiles]string
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

	f := &form{theme: winui.CurrentTheme()}
	f.bgBrush = winui.NewBrush(f.theme.Bg)
	f.titleFont = winui.Font(16, 600, false)
	f.bodyFont = winui.Font(13, 600, false)
	f.smallFont = winui.Font(12, 400, false)
	f.sectionFnt = winui.Font(11, 600, false)
	f.buttonFnt = winui.Font(13, 600, false)
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

	clientH := layoutHeight()
	style := uintptr(winui.WSCaption | winui.WSSysMenu)
	x, y, winW, winH := winui.CenteredFrame(winWidth, clientH, style)
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

func layoutHeight() int32 {
	return 680
}

func createBody(parent, instance windows.Handle, f *form) {
	y := int32(pad)
	if f.logoBMP != 0 {
		logo := winui.CreateControl(0, "STATIC", "", winui.WSChild|winui.WSVisible|winui.SSBitmap, pad, y, logoSize, logoSize, parent, instance, 0)
		winui.ProcSendMessageW.Call(uintptr(logo), winui.WMSetImage, winui.ImageBitmap, uintptr(f.logoBMP))
	}
	f.titleHWND = label(parent, instance, f.titleFont, "Kryptic", pad+logoSize+12, y, 240, 20)
	f.versionHWND = label(parent, instance, f.smallFont, "", pad+logoSize+12, y+20, 240, 16)
	y += logoSize + 14

	f.statusHWND = label(parent, instance, f.bodyFont, "", pad, y, winWidth-2*pad, 20)
	y += 22
	f.emailHWND = label(parent, instance, f.bodyFont, "", pad, y, winWidth-2*pad, 18)
	y += 18
	f.orgHWND = label(parent, instance, f.smallFont, "", pad, y, winWidth-2*pad, 16)
	y += 18
	f.apiHWND = label(parent, instance, f.smallFont, "", pad, y, winWidth-2*pad, 16)
	y += 18
	f.bannerHWND = label(parent, instance, f.smallFont, "", pad, y, winWidth-2*pad, 20)
	y += 24

	inner := winWidth - 2*pad
	y = section(parent, instance, f, "ACCOUNT", y)
	f.accountHWND = winui.CreateButton(parent, instance, idAccount, pad, y, inner, btnH, "Sign In", winui.ButtonPrimary, f.buttonFnt)
	y += btnH + gap
	f.addHWND = winui.CreateButton(parent, instance, idAdd, pad, y, inner, btnH, "Add Account", winui.ButtonGhost, f.buttonFnt)
	y += btnH + gap
	for i := 0; i < maxProfiles; i++ {
		f.profiles[i] = winui.CreateButton(parent, instance, uintptr(idProfile+i), pad, y, inner, btnH, "", winui.ButtonGhost, f.buttonFnt)
		winui.Show(f.profiles[i], false)
		y += btnH + gap
	}

	y = section(parent, instance, f, "OPERATIONS", y)
	f.flushHWND = winui.CreateButton(parent, instance, idFlush, pad, y, inner, btnH, "Refresh Secrets Cache", winui.ButtonGhost, f.buttonFnt)
	y += btnH + gap
	f.scanHWND = winui.CreateButton(parent, instance, idScan, pad, y, inner, btnH, "Scan for secrets", winui.ButtonGhost, f.buttonFnt)
	y += btnH + gap

	y = section(parent, instance, f, "SETTINGS", y)
	f.updateHWND = winui.CreateButton(parent, instance, idUpdate, pad, y, inner, btnH, "Check for Updates", winui.ButtonGhost, f.buttonFnt)
	y += btnH + gap
	f.serverHWND = winui.CreateButton(parent, instance, idServer, pad, y, inner, btnH, "Server URI", winui.ButtonGhost, f.buttonFnt)
	y += btnH + gap

	y = section(parent, instance, f, "HELP", y)
	third := (inner - 2*gap) / 3
	f.githubHWND = winui.CreateButton(parent, instance, idGitHub, pad, y, third, btnH, "GitHub", winui.ButtonGhost, f.buttonFnt)
	f.docsHWND = winui.CreateButton(parent, instance, idDocs, pad+third+gap, y, third, btnH, "Docs", winui.ButtonGhost, f.buttonFnt)
	f.logsHWND = winui.CreateButton(parent, instance, idLogs, pad+2*(third+gap), y, third, btnH, "Logs", winui.ButtonGhost, f.buttonFnt)
	y += btnH + gap
	f.aboutHWND = winui.CreateButton(parent, instance, idAbout, pad, y, inner, btnH, "About Kryptic", winui.ButtonGhost, f.buttonFnt)
	y += btnH + gap
	f.quitHWND = winui.CreateButton(parent, instance, idQuit, pad, y, inner, btnH, "Quit Kryptic", winui.ButtonDanger, f.buttonFnt)
}

func section(parent, instance windows.Handle, f *form, title string, y int32) int32 {
	label(parent, instance, f.sectionFnt, title, pad, y, winWidth-2*pad, 16)
	return y + 22
}

func label(parent, instance, font windows.Handle, text string, x, y, w, h int32) windows.Handle {
	hwnd := winui.CreateControl(0, "STATIC", text, winui.WSChild|winui.WSVisible|winui.SSEditCtrl, x, y, w, h, parent, instance, 0)
	winui.ProcSendMessageW.Call(uintptr(hwnd), winui.WMSetFont, uintptr(font), 1)
	return hwnd
}

func paint(f *form) {
	formMu.Lock()
	handlers := hnd
	formMu.Unlock()
	snap := handlers.snap()
	version := snap.Version
	if version == "" {
		version = DefaultVersion()
	}
	winui.SetText(f.versionHWND, "Version "+version)
	winui.SetText(f.statusHWND, statusGlyph(snap.Connection)+"  "+snap.ConnectionLabel)
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
		winui.SetText(f.profiles[shown], profile.Title())
		winui.Show(f.profiles[shown], true)
		winui.Enable(f.profiles[shown], !profile.Active)
		f.profileIDs[shown] = profile.ID
		shown++
	}
	for i := shown; i < maxProfiles; i++ {
		winui.Show(f.profiles[i], false)
		f.profileIDs[i] = ""
	}
}

func statusGlyph(kind string) string {
	switch kind {
	case "connected":
		return "●"
	case "connecting", "awaiting_approval":
		return "●"
	default:
		return "○"
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
		color := f.theme.Primary
		switch windows.Handle(lparam) {
		case f.versionHWND, f.orgHWND, f.apiHWND:
			color = f.theme.Secondary
		case f.bannerHWND:
			color = f.theme.Danger
		}
		return winui.PaintStatic(wparam, color, f.bgBrush)
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
		if id >= idProfile && id < idProfile+maxProfiles {
			i := int(id - idProfile)
			if f.profileIDs[i] != "" {
				handlers.fire("switchProfile", f.profileIDs[i])
			}
		}
	}
}
