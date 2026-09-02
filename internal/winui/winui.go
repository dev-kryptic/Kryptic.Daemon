//go:build windows

// Package winui is the shared Windows look for tray dialogs: same colors,
// logo, and window icon as the About panel.
package winui

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"github.com/dev-kryptic/daemon/internal/brand"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	WSVisible       = 0x10000000
	WSCaption       = 0x00C00000
	WSSysMenu       = 0x00080000
	WSChild         = 0x40000000
	WSTabStop       = 0x00010000
	WSBorder        = 0x00800000
	BSPushButton    = 0x00000000
	BSDefPushButton = 0x00000001
	BSOwnerDraw     = 0x0000000B
	SSCenter        = 0x00000001
	SSNotify        = 0x00000100
	SSBitmap        = 0x0000000E
	SSEditCtrl      = 0x00002000
	ESAutoHScroll   = 0x00000080
	SWShow          = 5

	WMDestroy        = 0x0002
	WMClose          = 0x0010
	WMEraseBkgnd     = 0x0014
	WMDrawItem       = 0x002B
	WMSetFont        = 0x0030
	WMMouseMove      = 0x0200
	WMMouseLeave     = 0x02A3
	WMCommand        = 0x0111
	WMSysCommand     = 0x0112
	WMCtlColorBtn    = 0x0135
	WMCtlColorEdit   = 0x0133
	WMCtlColorStatic = 0x0138
	WMSetImage       = 0x0172
	WMKeyDown        = 0x0100
	WMSetIcon        = 0x0080
	WMGetText        = 0x000D
	WMGetTextLength  = 0x000E
	WMUser           = 0x0400
	WMApp            = 0x8000
	PBMSetPos        = WMUser + 2
	PBMSetRange32    = WMUser + 6
	PBSSmooth        = 1

	IconSmall   = 0
	IconBig     = 1
	ImageBitmap = 0
	IDOK        = 1
	IDCancel    = 2

	VKReturn = 0x0D
	VKEscape = 0x1B

	bkTransparent = 1
	idcArrow      = 32512

	dwmwaUseImmersiveDarkMode = 20
	GWLWNDPROC                = ^uintptr(3)
	GWLPUserData              = ^uintptr(20)

	ButtonPrimary uintptr = 1
	ButtonGhost   uintptr = 2

	odsSelected = 0x0001
	odsFocus    = 0x0010
	psSolid     = 0
	dtCenter    = 0x00000001
	dtVCenter   = 0x00000004
	dtSingle    = 0x00000020
	tmeLeave    = 0x00000002
	buttonRadius = 20
)

var (
	User32   = windows.NewLazySystemDLL("user32.dll")
	Gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	Kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	Dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")

	ProcRegisterClassExW     = User32.NewProc("RegisterClassExW")
	ProcCreateWindowExW      = User32.NewProc("CreateWindowExW")
	ProcDefWindowProcW       = User32.NewProc("DefWindowProcW")
	ProcDestroyWindow        = User32.NewProc("DestroyWindow")
	ProcShowWindow           = User32.NewProc("ShowWindow")
	ProcUpdateWindow         = User32.NewProc("UpdateWindow")
	ProcGetMessageW          = User32.NewProc("GetMessageW")
	ProcTranslateMessage     = User32.NewProc("TranslateMessage")
	ProcDispatchMessageW     = User32.NewProc("DispatchMessageW")
	ProcGetSystemMetrics     = User32.NewProc("GetSystemMetrics")
	ProcSetForegroundWindow  = User32.NewProc("SetForegroundWindow")
	ProcLoadCursorW          = User32.NewProc("LoadCursorW")
	ProcSendMessageW         = User32.NewProc("SendMessageW")
	ProcGetDC                = User32.NewProc("GetDC")
	ProcReleaseDC            = User32.NewProc("ReleaseDC")
	ProcPostQuitMessage      = User32.NewProc("PostQuitMessage")
	ProcGetClientRect        = User32.NewProc("GetClientRect")
	ProcFillRect             = User32.NewProc("FillRect")
	ProcAdjustWindowRect     = User32.NewProc("AdjustWindowRect")
	ProcSetWindowLongPtrW    = User32.NewProc("SetWindowLongPtrW")
	ProcGetWindowLongPtrW    = User32.NewProc("GetWindowLongPtrW")
	ProcCallWindowProcW      = User32.NewProc("CallWindowProcW")
	ProcInvalidateRect       = User32.NewProc("InvalidateRect")
	ProcTrackMouseEvent      = User32.NewProc("TrackMouseEvent")
	ProcDrawTextW            = User32.NewProc("DrawTextW")
	ProcGetWindowTextW       = User32.NewProc("GetWindowTextW")
	ProcGetWindowTextLengthW = User32.NewProc("GetWindowTextLengthW")
	ProcSetFocus             = User32.NewProc("SetFocus")
	ProcGetParent            = User32.NewProc("GetParent")
	ProcGetDlgCtrlID         = User32.NewProc("GetDlgCtrlID")
	ProcSetWindowTextW       = User32.NewProc("SetWindowTextW")
	ProcPostMessageW         = User32.NewProc("PostMessageW")

	ProcGetModuleHandleW   = Kernel32.NewProc("GetModuleHandleW")
	ProcCreateSolidBrush   = Gdi32.NewProc("CreateSolidBrush")
	ProcCreatePen          = Gdi32.NewProc("CreatePen")
	ProcSelectObject       = Gdi32.NewProc("SelectObject")
	ProcRoundRect          = Gdi32.NewProc("RoundRect")
	ProcCreateFontW        = Gdi32.NewProc("CreateFontW")
	ProcCreateDIBSection   = Gdi32.NewProc("CreateDIBSection")
	ProcDeleteObject       = Gdi32.NewProc("DeleteObject")
	ProcSetBkMode          = Gdi32.NewProc("SetBkMode")
	ProcSetBkColor         = Gdi32.NewProc("SetBkColor")
	ProcSetTextColor       = Gdi32.NewProc("SetTextColor")
	ProcCreateBitmap       = Gdi32.NewProc("CreateBitmap")
	ProcCreateIconIndirect = User32.NewProc("CreateIconIndirect")
	ProcDestroyIcon        = User32.NewProc("DestroyIcon")

	ProcDwmSetWindowAttribute = Dwmapi.NewProc("DwmSetWindowAttribute")
)

// Theme is COLORREF (0x00BBGGRR), matching the About panel.
type Theme struct {
	Dark       bool
	Bg         uint32
	Primary    uint32
	Secondary  uint32
	Tertiary   uint32
	Link       uint32
	Field      uint32
	Accent     uint32
	AccentHot  uint32
	AccentDown uint32
	Ink        uint32
	Surface    uint32
	SurfaceHot uint32
	Border     uint32
	BorderHot  uint32
}

func CurrentTheme() Theme {
	if appsUseLightTheme() {
		return Theme{
			Dark:       false,
			Bg:         0x00FFFFFF,
			Primary:    0x001A1A1A,
			Secondary:  0x0080726B,
			Tertiary:   0x00AFA39C,
			Link:       0x00C06700,
			Field:      0x00FFFFFF,
			Accent:     0x0069B016, // #16b069
			AccentHot:  0x0078C025,
			AccentDown: 0x00579A0C, // #0c9a57
			Ink:        0x000C1404, // #04140c
			Surface:    0x00FFFFFF,
			SurfaceHot: 0x00F4F6F3,
			Border:     0x00E0E6DD, // #dde6e0
			BorderHot:  0x00C8D0C5,
		}
	}
	return Theme{
		Dark:       true,
		Bg:         0x00202020,
		Primary:    0x00F0F0F0,
		Secondary:  0x00B8ADA6,
		Tertiary:   0x008E827A,
		Link:       0x00FFC24C,
		Field:      0x002C2C2C,
		Accent:     0x00A6F25F, // #5ff2a6
		AccentHot:  0x00B5F578,
		AccentDown: 0x0086D43B, // #3bd486
		Ink:        0x000C1404, // #04140c
		Surface:    0x002C2C2C,
		SurfaceHot: 0x00353535,
		Border:     0x003A3A3A,
		BorderHot:  0x00555555,
	}
}

func appsUseLightTheme() bool {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return true
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	return err != nil || value == 1
}

type WndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type Msg struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type Rect struct {
	Left, Top, Right, Bottom int32
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  windows.Handle
	HbmColor windows.Handle
}

func Instance() windows.Handle {
	h, _, _ := ProcGetModuleHandleW.Call(0)
	return windows.Handle(h)
}

func ArrowCursor() windows.Handle {
	h, _, _ := ProcLoadCursorW.Call(0, idcArrow)
	return windows.Handle(h)
}

func NewBrush(color uint32) windows.Handle {
	h, _, _ := ProcCreateSolidBrush.Call(uintptr(color))
	return windows.Handle(h)
}

var (
	iconMu    sync.Mutex
	iconSmall windows.Handle
	iconBig   windows.Handle
)

func AppIcons() (small, big windows.Handle) {
	iconMu.Lock()
	defer iconMu.Unlock()
	if iconSmall == 0 {
		iconSmall = hiconFromPNG(brand.LogoPNG, 16)
	}
	if iconBig == 0 {
		iconBig = hiconFromPNG(brand.LogoPNG, 32)
	}
	return iconSmall, iconBig
}

func ApplyChrome(hwnd windows.Handle, dark bool) {
	small, big := AppIcons()
	if big != 0 {
		ProcSendMessageW.Call(uintptr(hwnd), WMSetIcon, IconBig, uintptr(big))
	}
	if small != 0 {
		ProcSendMessageW.Call(uintptr(hwnd), WMSetIcon, IconSmall, uintptr(small))
	}
	v := int32(0)
	if dark {
		v = 1
	}
	ProcDwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&v)), 4)
}

func Font(px, weight int32, underline bool) windows.Handle {
	underlined := uintptr(0)
	if underline {
		underlined = 1
	}
	face, _ := windows.UTF16PtrFromString("Segoe UI")
	h, _, _ := ProcCreateFontW.Call(
		uintptr(px), 0, 0, 0, uintptr(weight), 0, underlined, 0,
		1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(face)),
	)
	return windows.Handle(h)
}

func CreateControl(exStyle uint32, class, text string, style uint32, x, y, w, h int32, parent, instance windows.Handle, id uintptr) windows.Handle {
	classPtr, _ := windows.UTF16PtrFromString(class)
	textPtr, _ := windows.UTF16PtrFromString(text)
	hwnd, _, _ := ProcCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), id, uintptr(instance), 0,
	)
	return windows.Handle(hwnd)
}

func CenteredFrame(clientW, clientH int32, style uintptr) (x, y, winW, winH int32) {
	frame := Rect{0, 0, clientW, clientH}
	ProcAdjustWindowRect.Call(uintptr(unsafe.Pointer(&frame)), style, 0)
	winW = frame.Right - frame.Left
	winH = frame.Bottom - frame.Top
	screenW, _, _ := ProcGetSystemMetrics.Call(0)
	screenH, _, _ := ProcGetSystemMetrics.Call(1)
	x = (int32(screenW) - winW) / 2
	y = (int32(screenH) - winH) / 2
	return
}

func RunModal() {
	var m Msg
	for {
		ret, _, _ := ProcGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		ProcTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		ProcDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func WindowText(hwnd windows.Handle) string {
	n, _, _ := ProcGetWindowTextLengthW.Call(uintptr(hwnd))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	ProcGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(n+1))
	return windows.UTF16ToString(buf)
}

func FillBackground(hdc uintptr, hwnd uintptr, brush windows.Handle) uintptr {
	var rc Rect
	ProcGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	ProcFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(brush))
	return 1
}

func PaintStatic(hdc uintptr, color uint32, brush windows.Handle) uintptr {
	ProcSetBkMode.Call(hdc, bkTransparent)
	ProcSetTextColor.Call(hdc, uintptr(color))
	return uintptr(brush)
}

func PaintEdit(hdc uintptr, text, field uint32, fieldBrush windows.Handle) uintptr {
	ProcSetBkMode.Call(hdc, 2) // OPAQUE
	ProcSetTextColor.Call(hdc, uintptr(text))
	ProcSetBkColor.Call(hdc, uintptr(field))
	return uintptr(fieldBrush)
}

func LogoBitmap(size int, bg uint32) (windows.Handle, error) {
	return hbitmapFromPNG(brand.LogoPNG, size, size, bg)
}

func hiconFromPNG(data []byte, size int) windows.Handle {
	color, err := hbitmapFromPNG(data, size, size, 0x00000000)
	if err != nil || color == 0 {
		return 0
	}
	mask, _, _ := ProcCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, 0)
	if mask == 0 {
		return 0
	}
	info := iconInfo{
		FIcon:    1,
		XHotspot: uint32(size / 2),
		YHotspot: uint32(size / 2),
		HbmMask:  windows.Handle(mask),
		HbmColor: color,
	}
	h, _, _ := ProcCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	ProcDeleteObject.Call(uintptr(color))
	ProcDeleteObject.Call(mask)
	return windows.Handle(h)
}

func hbitmapFromPNG(data []byte, maxW, maxH int, bg uint32) (windows.Handle, error) {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	scaled := scaleToFit(src, maxW, maxH)
	bounds := scaled.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	header := bitmapInfoHeader{
		Size:        40,
		Width:       int32(w),
		Height:      int32(-h),
		Planes:      1,
		BitCount:    32,
		Compression: 0,
	}
	hdc, _, _ := ProcGetDC.Call(0)
	defer ProcReleaseDC.Call(0, hdc)

	var bits unsafe.Pointer
	bmp, _, _ := ProcCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&header)),
		0,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if bmp == 0 || bits == nil {
		return 0, syscall.EINVAL
	}

	bgR := int(byte(bg))
	bgG := int(byte(bg >> 8))
	bgB := int(byte(bg >> 16))
	dst := unsafe.Slice((*byte)(bits), w*h*4)
	for i := 0; i < w*h; i++ {
		r := int(scaled.Pix[i*4+0])
		g := int(scaled.Pix[i*4+1])
		b := int(scaled.Pix[i*4+2])
		a := int(scaled.Pix[i*4+3])
		if bg == 0 {
			dst[i*4+0] = byte(b)
			dst[i*4+1] = byte(g)
			dst[i*4+2] = byte(r)
			dst[i*4+3] = byte(a)
			continue
		}
		dst[i*4+0] = byte((b*a + bgB*(255-a)) / 255)
		dst[i*4+1] = byte((g*a + bgG*(255-a)) / 255)
		dst[i*4+2] = byte((r*a + bgR*(255-a)) / 255)
		dst[i*4+3] = 255
	}
	return windows.Handle(bmp), nil
}

func scaleToFit(src image.Image, maxW, maxH int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := math.Min(float64(maxW)/float64(w), float64(maxH)/float64(h))
	if scale > 1 {
		scale = 1
	}
	nw := int(math.Max(1, float64(w)*scale))
	nh := int(math.Max(1, float64(h)*scale))
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			out.Set(x, y, src.At(b.Min.X+x*w/nw, b.Min.Y+y*h/nh))
		}
	}
	return out
}
