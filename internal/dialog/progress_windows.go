//go:build windows

package dialog

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/dev-kryptic/daemon/internal/winui"
	"golang.org/x/sys/windows"
)

const (
	progressWidth = 420
	idProgressBar = 1010
	idStatusText  = 1011
	idPercentText = 1012
	wmFinish      = winui.WMApp + 1
)

var (
	progressClass  = windows.StringToUTF16Ptr("KrypticProgress")
	progressCB     = windows.NewCallback(progressWndProc)
	progressMu     sync.Mutex
	progressByHWND = map[uintptr]*winProgress{}
	commonControls sync.Once
	procInitCC     = windows.NewLazySystemDLL("comctl32.dll").NewProc("InitCommonControlsEx")
)

type initCommonControlsEx struct {
	size uint32
	icc  uint32
}

type winProgress struct {
	mu         sync.Mutex
	hwnd       windows.Handle
	bar        windows.Handle
	status     windows.Handle
	percent    windows.Handle
	canceled   chan struct{}
	done       chan struct{}
	ready      chan struct{}
	cancelOnce sync.Once
	closeOnce  sync.Once
	closed     bool
	theme      winui.Theme
	bgBrush    windows.Handle
	bodyFnt    windows.Handle
	buttonFnt  windows.Handle
}

func OpenProgress(title, message string) Progress {
	initProgressControls()
	p := &winProgress{
		canceled: make(chan struct{}),
		done:     make(chan struct{}),
		ready:    make(chan struct{}),
	}
	go func() {
		runtime.LockOSThread()
		defer close(p.done)
		p.run(title, message)
	}()
	select {
	case <-p.ready:
	case <-time.After(2 * time.Second):
		return nopProgress{}
	}
	p.mu.Lock()
	hwnd := p.hwnd
	p.mu.Unlock()
	if hwnd == 0 {
		return nopProgress{}
	}
	return p
}

func initProgressControls() {
	commonControls.Do(func() {
		info := initCommonControlsEx{size: 8, icc: 0x00000020} // ICC_PROGRESS_CLASS
		procInitCC.Call(uintptr(unsafe.Pointer(&info)))
	})
}

func (p *winProgress) Set(percent int, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.hwnd == 0 {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	winui.ProcSendMessageW.Call(uintptr(p.bar), winui.PBMSetPos, uintptr(percent), 0)
	if p.percent != 0 {
		text, _ := windows.UTF16PtrFromString(fmt.Sprintf("%d%%", percent))
		winui.ProcSetWindowTextW.Call(uintptr(p.percent), uintptr(unsafe.Pointer(text)))
	}
	if p.status != 0 && message != "" {
		text, _ := windows.UTF16PtrFromString(message)
		winui.ProcSetWindowTextW.Call(uintptr(p.status), uintptr(unsafe.Pointer(text)))
	}
}

func (p *winProgress) Close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		hwnd := p.hwnd
		p.closed = true
		p.mu.Unlock()
		if hwnd != 0 {
			winui.ProcPostMessageW.Call(uintptr(hwnd), wmFinish, 0, 0)
		}
		<-p.done
	})
}

func (p *winProgress) Canceled() <-chan struct{} {
	return p.canceled
}

func (p *winProgress) signalCancel() {
	p.cancelOnce.Do(func() { close(p.canceled) })
}

func (p *winProgress) run(title, message string) {
	p.theme = winui.CurrentTheme()
	p.bgBrush = winui.NewBrush(p.theme.Bg)
	p.bodyFnt = winui.Font(14, 400, false)
	p.buttonFnt = winui.Font(14, 400, false)

	instance := winui.Instance()
	small, big := winui.AppIcons()
	class := winui.WndClassEx{
		Size:      uint32(unsafe.Sizeof(winui.WndClassEx{})),
		WndProc:   progressCB,
		Instance:  instance,
		Cursor:    winui.ArrowCursor(),
		Icon:      big,
		IconSm:    small,
		ClassName: progressClass,
	}
	winui.ProcRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))

	clientH := int32(24 + 28 + 16 + 22 + 12 + 22 + 20 + 32 + 24)
	style := uintptr(winui.WSCaption | winui.WSSysMenu)
	x, y, winW, winH := winui.CenteredFrame(progressWidth, clientH, style)
	titlePtr, _ := windows.UTF16PtrFromString(title)
	hwnd, _, _ := winui.ProcCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(progressClass)),
		uintptr(unsafe.Pointer(titlePtr)),
		style,
		uintptr(x), uintptr(y), uintptr(winW), uintptr(winH),
		0, 0, uintptr(instance), 0,
	)
	p.mu.Lock()
	p.hwnd = windows.Handle(hwnd)
	p.mu.Unlock()
	close(p.ready)
	if hwnd == 0 {
		return
	}

	progressMu.Lock()
	progressByHWND[hwnd] = p
	progressMu.Unlock()
	defer func() {
		progressMu.Lock()
		delete(progressByHWND, hwnd)
		progressMu.Unlock()
	}()

	winui.ApplyChrome(windows.Handle(hwnd), p.theme.Dark)
	cy := int32(24)
	status := winui.CreateControl(0, "STATIC", message, winui.WSChild|winui.WSVisible|winui.SSCenter|winui.SSEditCtrl, 28, cy, progressWidth-56, 28, windows.Handle(hwnd), instance, idStatusText)
	winui.ProcSendMessageW.Call(uintptr(status), winui.WMSetFont, uintptr(p.bodyFnt), 1)
	p.status = status
	cy += 40

	bar := winui.CreateControl(0, "msctls_progress32", "", winui.WSChild|winui.WSVisible|winui.PBSSmooth, 28, cy, progressWidth-56, 22, windows.Handle(hwnd), instance, idProgressBar)
	winui.ProcSendMessageW.Call(uintptr(bar), winui.PBMSetRange32, 0, 100)
	p.bar = bar
	cy += 34

	percent := winui.CreateControl(0, "STATIC", "0%", winui.WSChild|winui.WSVisible|winui.SSCenter, 28, cy, progressWidth-56, 22, windows.Handle(hwnd), instance, idPercentText)
	winui.ProcSendMessageW.Call(uintptr(percent), winui.WMSetFont, uintptr(p.bodyFnt), 1)
	p.percent = percent
	cy += 36

	cancel := winui.CreateControl(0, "BUTTON", "Cancel",
		winui.WSChild|winui.WSVisible|winui.WSTabStop|winui.BSPushButton,
		146, cy, 128, 32, windows.Handle(hwnd), instance, winui.IDCancel)
	winui.ProcSendMessageW.Call(uintptr(cancel), winui.WMSetFont, uintptr(p.buttonFnt), 1)

	winui.ProcShowWindow.Call(hwnd, winui.SWShow)
	winui.ProcUpdateWindow.Call(hwnd)
	winui.ProcSetForegroundWindow.Call(hwnd)
	winui.RunModal()
}

func progressFromHWND(hwnd uintptr) *winProgress {
	progressMu.Lock()
	defer progressMu.Unlock()
	return progressByHWND[hwnd]
}

func progressWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	p := progressFromHWND(hwnd)
	if p == nil {
		ret, _, _ := winui.ProcDefWindowProcW.Call(hwnd, message, wparam, lparam)
		return ret
	}

	switch message {
	case winui.WMEraseBkgnd:
		return winui.FillBackground(wparam, hwnd, p.bgBrush)
	case winui.WMCtlColorStatic:
		return winui.PaintStatic(wparam, p.theme.Secondary, p.bgBrush)
	case winui.WMCommand:
		if wparam&0xffff == winui.IDCancel {
			p.signalCancel()
			winui.ProcDestroyWindow.Call(hwnd)
			return 0
		}
	case winui.WMKeyDown:
		if wparam == winui.VKEscape {
			p.signalCancel()
			winui.ProcDestroyWindow.Call(hwnd)
			return 0
		}
	case winui.WMClose:
		p.signalCancel()
		winui.ProcDestroyWindow.Call(hwnd)
		return 0
	case wmFinish:
		winui.ProcDestroyWindow.Call(hwnd)
		return 0
	case winui.WMDestroy:
		winui.ProcPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := winui.ProcDefWindowProcW.Call(hwnd, message, wparam, lparam)
	return ret
}
