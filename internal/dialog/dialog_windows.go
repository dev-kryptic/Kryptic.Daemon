//go:build windows

package dialog

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/dev-kryptic/daemon/internal/winui"
	"golang.org/x/sys/windows"
)

const (
	dlgWidth = 420
	logoSize = 72
	idOK     = winui.IDOK
	idCancel = winui.IDCancel
	idEdit   = 1003
)

type dlgKind int

const (
	kindInfo dlgKind = iota
	kindConfirm
	kindPrompt
	kindAsk
)

type dlg struct {
	kind      dlgKind
	title     string
	message   string
	value     string
	theme     winui.Theme
	bgBrush   windows.Handle
	fieldBr   windows.Handle
	logoBMP   windows.Handle
	titleFnt  windows.Handle
	bodyFnt   windows.Handle
	buttonFnt windows.Handle
	editHWND  windows.Handle
	okHWND    windows.Handle
	editPrev  uintptr
	ok        bool
	closed    bool
	accept    string
	reject    string
	msgH      int32
}

var (
	dlgClass = windows.StringToUTF16Ptr("KrypticDialog")
	dlgCB    = windows.NewCallback(dlgWndProc)
	editCB   = windows.NewCallback(editWndProc)
	dlgMu    sync.Mutex
	active   *dlg
)

func Info(title, message string) {
	run(kindInfo, title, message, "")
}

func Confirm(title, message string) bool {
	d := run(kindConfirm, title, message, "")
	return d.ok
}

func Ask(title, message, accept, reject string) bool {
	d := runAsk(title, message, accept, reject)
	return d.ok
}

func Prompt(title, message, defaultValue string) (string, bool) {
	d := run(kindPrompt, title, message, defaultValue)
	if !d.ok {
		return "", false
	}
	return d.value, true
}

func run(kind dlgKind, title, message, defaultValue string) *dlg {
	d := &dlg{
		kind:    kind,
		title:   title,
		message: message,
		value:   defaultValue,
		theme:   winui.CurrentTheme(),
		msgH:    48,
	}
	return present(d)
}

func runAsk(title, message, accept, reject string) *dlg {
	if accept == "" {
		accept = "OK"
	}
	if reject == "" {
		reject = "Close"
	}
	d := &dlg{
		kind:    kindAsk,
		title:   title,
		message: message,
		theme:   winui.CurrentTheme(),
		accept:  accept,
		reject:  reject,
		msgH:    120,
	}
	return present(d)
}

func present(d *dlg) *dlg {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	d.bgBrush = winui.NewBrush(d.theme.Bg)
	d.fieldBr = winui.NewBrush(d.theme.Field)
	if bmp, err := winui.LogoBitmap(logoSize, d.theme.Bg); err == nil {
		d.logoBMP = bmp
	}
	d.titleFnt = winui.Font(16, 600, false)
	d.bodyFnt = winui.Font(14, 400, false)
	d.buttonFnt = winui.Font(14, 400, false)

	dlgMu.Lock()
	active = d
	dlgMu.Unlock()
	defer func() {
		dlgMu.Lock()
		active = nil
		dlgMu.Unlock()
	}()

	instance := winui.Instance()
	small, big := winui.AppIcons()
	class := winui.WndClassEx{
		Size:      uint32(unsafe.Sizeof(winui.WndClassEx{})),
		WndProc:   dlgCB,
		Instance:  instance,
		Cursor:    winui.ArrowCursor(),
		Icon:      big,
		IconSm:    small,
		ClassName: dlgClass,
	}
	winui.ProcRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))

	clientH := contentHeight(d)
	style := uintptr(winui.WSCaption | winui.WSSysMenu)
	x, y, winW, winH := winui.CenteredFrame(dlgWidth, clientH, style)
	titlePtr, _ := windows.UTF16PtrFromString(d.title)
	hwnd, _, _ := winui.ProcCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(dlgClass)),
		uintptr(unsafe.Pointer(titlePtr)),
		style|winui.WSVisible,
		uintptr(x), uintptr(y), uintptr(winW), uintptr(winH),
		0, 0, uintptr(instance), 0,
	)
	if hwnd == 0 {
		return d
	}
	winui.ApplyChrome(windows.Handle(hwnd), d.theme.Dark)
	createBody(windows.Handle(hwnd), instance, d)
	winui.ProcShowWindow.Call(hwnd, winui.SWShow)
	winui.ProcUpdateWindow.Call(hwnd)
	winui.ProcSetForegroundWindow.Call(hwnd)
	if d.editHWND != 0 {
		winui.ProcSetFocus.Call(uintptr(d.editHWND))
	} else if d.okHWND != 0 {
		winui.ProcSetFocus.Call(uintptr(d.okHWND))
	} else {
		winui.ProcSetFocus.Call(hwnd)
	}
	winui.RunModal()
	return d
}

func contentHeight(d *dlg) int32 {
	msgH := d.msgH
	if msgH == 0 {
		msgH = 48
	}
	h := int32(24 + logoSize + 16 + msgH + 16 + 32 + 24)
	if d.kind == kindPrompt {
		h += 36
	}
	return h
}

func createBody(parent, instance windows.Handle, d *dlg) {
	y := int32(24)
	if d.logoBMP != 0 {
		x := int32((dlgWidth - logoSize) / 2)
		logo := winui.CreateControl(0, "STATIC", "", winui.WSChild|winui.WSVisible|winui.SSBitmap, x, y, logoSize, logoSize, parent, instance, 0)
		winui.ProcSendMessageW.Call(uintptr(logo), winui.WMSetImage, winui.ImageBitmap, uintptr(d.logoBMP))
		y += logoSize + 16
	}

	msgH := d.msgH
	if msgH == 0 {
		msgH = 48
	}
	msg := winui.CreateControl(0, "STATIC", d.message, winui.WSChild|winui.WSVisible|winui.SSCenter|winui.SSEditCtrl, 28, y, dlgWidth-56, msgH, parent, instance, 0)
	winui.ProcSendMessageW.Call(uintptr(msg), winui.WMSetFont, uintptr(d.bodyFnt), 1)
	y += msgH + 8

	if d.kind == kindPrompt {
		d.editHWND = winui.CreateControl(0x00000200, "EDIT", d.value, winui.WSChild|winui.WSVisible|winui.WSTabStop|winui.ESAutoHScroll, 28, y, dlgWidth-56, 28, parent, instance, idEdit)
		winui.ProcSendMessageW.Call(uintptr(d.editHWND), winui.WMSetFont, uintptr(d.bodyFnt), 1)
		d.editPrev, _, _ = winui.ProcSetWindowLongPtrW.Call(uintptr(d.editHWND), winui.GWLWNDPROC, editCB)
		y += 40
	}

	okLabel := d.accept
	if okLabel == "" {
		okLabel = "OK"
		if d.kind == kindConfirm {
			okLabel = "Continue"
		}
	}

	const buttonW, buttonH, gap int32 = 128, 32, 12
	if d.kind == kindInfo {
		x := (dlgWidth - buttonW) / 2
		d.okHWND = winui.CreateControl(0, "BUTTON", okLabel,
			winui.WSChild|winui.WSVisible|winui.WSTabStop|winui.BSDefPushButton,
			x, y, buttonW, buttonH, parent, instance, idOK)
		winui.ProcSendMessageW.Call(uintptr(d.okHWND), winui.WMSetFont, uintptr(d.buttonFnt), 1)
		return
	}

	cancelLabel := d.reject
	if cancelLabel == "" {
		cancelLabel = "Cancel"
	}
	total := buttonW*2 + gap
	start := (dlgWidth - total) / 2
	d.okHWND = winui.CreateControl(0, "BUTTON", okLabel,
		winui.WSChild|winui.WSVisible|winui.WSTabStop|winui.BSDefPushButton,
		start, y, buttonW, buttonH, parent, instance, idOK)
	winui.ProcSendMessageW.Call(uintptr(d.okHWND), winui.WMSetFont, uintptr(d.buttonFnt), 1)
	cancel := winui.CreateControl(0, "BUTTON", cancelLabel,
		winui.WSChild|winui.WSVisible|winui.WSTabStop|winui.BSPushButton,
		start+buttonW+gap, y, buttonW, buttonH, parent, instance, idCancel)
	winui.ProcSendMessageW.Call(uintptr(cancel), winui.WMSetFont, uintptr(d.buttonFnt), 1)
}

func dlgWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	dlgMu.Lock()
	d := active
	dlgMu.Unlock()
	if d == nil {
		ret, _, _ := winui.ProcDefWindowProcW.Call(hwnd, message, wparam, lparam)
		return ret
	}

	switch message {
	case winui.WMEraseBkgnd:
		return winui.FillBackground(wparam, hwnd, d.bgBrush)
	case winui.WMCtlColorStatic:
		return winui.PaintStatic(wparam, d.theme.Secondary, d.bgBrush)
	case winui.WMCtlColorEdit:
		return winui.PaintEdit(wparam, d.theme.Primary, d.theme.Field, d.fieldBr)
	case winui.WMCommand:
		switch wparam & 0xffff {
		case idOK:
			finish(d, hwnd, true)
			return 0
		case idCancel:
			finish(d, hwnd, false)
			return 0
		}
	case winui.WMKeyDown:
		if wparam == winui.VKEscape {
			finish(d, hwnd, d.kind == kindInfo)
			return 0
		}
		if wparam == winui.VKReturn && d.kind != kindPrompt {
			finish(d, hwnd, true)
			return 0
		}
	case winui.WMClose:
		finish(d, hwnd, false)
		return 0
	case winui.WMDestroy:
		winui.ProcPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := winui.ProcDefWindowProcW.Call(hwnd, message, wparam, lparam)
	return ret
}

func editWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	dlgMu.Lock()
	d := active
	dlgMu.Unlock()
	if d == nil {
		return 0
	}
	if message == winui.WMKeyDown {
		switch wparam {
		case winui.VKReturn:
			parent, _, _ := winui.ProcGetParent.Call(hwnd)
			finish(d, parent, true)
			return 0
		case winui.VKEscape:
			parent, _, _ := winui.ProcGetParent.Call(hwnd)
			finish(d, parent, false)
			return 0
		}
	}
	ret, _, _ := winui.ProcCallWindowProcW.Call(d.editPrev, hwnd, message, wparam, lparam)
	return ret
}

func finish(d *dlg, hwnd uintptr, ok bool) {
	if d.closed {
		return
	}
	d.closed = true
	if d.editHWND != 0 {
		d.value = winui.WindowText(d.editHWND)
	}
	d.ok = ok
	winui.ProcDestroyWindow.Call(hwnd)
}
