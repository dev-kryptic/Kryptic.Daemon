//go:build windows

package winui

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type drawItemStruct struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   windows.Handle
	HDC        windows.Handle
	RcItem     Rect
	ItemData   uintptr
}

type trackMouseEvent struct {
	Size      uint32
	Flags     uint32
	HwndTrack windows.Handle
	HoverTime uint32
}

var (
	buttonCB    = windows.NewCallback(buttonWndProc)
	buttonMu    sync.Mutex
	buttonPrev  = map[uintptr]uintptr{}
	buttonHover = map[uintptr]bool{}
)

// CreateButton makes an owner-drawn action that matches the rest of the
// Kryptic window instead of the default Win32 chrome.
func CreateButton(parent, instance windows.Handle, id uintptr, x, y, w, h int32, label string, kind uintptr, font windows.Handle) windows.Handle {
	hwnd := CreateControl(0, "BUTTON", label, WSChild|WSVisible|WSTabStop|BSOwnerDraw, x, y, w, h, parent, instance, id)
	if hwnd == 0 {
		return 0
	}
	ProcSetWindowLongPtrW.Call(uintptr(hwnd), GWLPUserData, kind)
	if font != 0 {
		ProcSendMessageW.Call(uintptr(hwnd), WMSetFont, uintptr(font), 1)
	}
	prev, _, _ := ProcSetWindowLongPtrW.Call(uintptr(hwnd), GWLWNDPROC, buttonCB)
	buttonMu.Lock()
	buttonPrev[uintptr(hwnd)] = prev
	buttonMu.Unlock()
	return hwnd
}

// HandleDrawItem paints a CreateButton control. Call it from the parent
// WM_DRAWITEM handler.
func HandleDrawItem(lparam uintptr, theme Theme, font windows.Handle) uintptr {
	dis := (*drawItemStruct)(unsafe.Pointer(lparam))
	kind, _, _ := ProcGetWindowLongPtrW.Call(uintptr(dis.HwndItem), GWLPUserData)
	pressed := dis.ItemState&odsSelected != 0
	focused := dis.ItemState&odsFocus != 0
	buttonMu.Lock()
	hover := buttonHover[uintptr(dis.HwndItem)]
	buttonMu.Unlock()
	paintButton(uintptr(dis.HDC), dis.RcItem, WindowText(dis.HwndItem), font, theme, kind, pressed, hover, focused)
	return 1
}

func buttonWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	buttonMu.Lock()
	prev := buttonPrev[hwnd]
	buttonMu.Unlock()

	switch message {
	case WMMouseMove:
		buttonMu.Lock()
		already := buttonHover[hwnd]
		if !already {
			buttonHover[hwnd] = true
		}
		buttonMu.Unlock()
		if !already {
			tme := trackMouseEvent{Size: uint32(unsafe.Sizeof(trackMouseEvent{})), Flags: tmeLeave, HwndTrack: windows.Handle(hwnd)}
			ProcTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			ProcInvalidateRect.Call(hwnd, 0, 0)
		}
	case WMMouseLeave:
		buttonMu.Lock()
		buttonHover[hwnd] = false
		buttonMu.Unlock()
		ProcInvalidateRect.Call(hwnd, 0, 0)
	case WMDestroy:
		buttonMu.Lock()
		delete(buttonHover, hwnd)
		delete(buttonPrev, hwnd)
		buttonMu.Unlock()
	}
	if prev == 0 {
		ret, _, _ := ProcDefWindowProcW.Call(hwnd, message, wparam, lparam)
		return ret
	}
	ret, _, _ := ProcCallWindowProcW.Call(prev, hwnd, message, wparam, lparam)
	return ret
}

func paintButton(hdc uintptr, rc Rect, label string, font windows.Handle, theme Theme, kind uintptr, pressed, hover, focused bool) {
	bg := NewBrush(theme.Bg)
	ProcFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(bg))
	ProcDeleteObject.Call(uintptr(bg))

	fill, border, ink := buttonColors(theme, kind, pressed, hover, focused)
	brush := NewBrush(fill)
	pen, _, _ := ProcCreatePen.Call(psSolid, 1, uintptr(border))
	oldBrush, _, _ := ProcSelectObject.Call(hdc, uintptr(brush))
	oldPen, _, _ := ProcSelectObject.Call(hdc, pen)
	ProcRoundRect.Call(hdc, uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom), buttonRadius, buttonRadius)
	ProcSelectObject.Call(hdc, oldBrush)
	ProcSelectObject.Call(hdc, oldPen)
	ProcDeleteObject.Call(uintptr(brush))
	ProcDeleteObject.Call(pen)

	if font != 0 {
		ProcSelectObject.Call(hdc, uintptr(font))
	}
	ProcSetBkMode.Call(hdc, bkTransparent)
	ProcSetTextColor.Call(hdc, uintptr(ink))
	text, _ := windows.UTF16FromString(label)
	ProcDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)-1), uintptr(unsafe.Pointer(&rc)), dtCenter|dtVCenter|dtSingle)
}

func buttonColors(theme Theme, kind uintptr, pressed, hover, focused bool) (fill, border, ink uint32) {
	if kind == ButtonPrimary {
		ink = theme.Ink
		fill = theme.Accent
		if hover {
			fill = theme.AccentHot
		}
		if pressed {
			fill = theme.AccentDown
		}
		border = fill
		if focused && !pressed {
			border = theme.AccentDown
		}
		return fill, border, ink
	}

	ink = theme.Primary
	fill = theme.Surface
	border = theme.Border
	if hover {
		fill = theme.SurfaceHot
		border = theme.BorderHot
	}
	if pressed {
		fill = theme.Field
	}
	if focused {
		border = theme.Accent
	}
	return fill, border, ink
}
