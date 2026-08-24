//go:build windows

package about

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/dev-kryptic/daemon/internal/winui"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	aboutWidth = 380
	logoSize   = 128
	idLink     = 1001

	wsVisible  = 0x10000000
	wsCaption  = 0x00C00000
	wsSysMenu  = 0x00080000
	wsChild    = 0x40000000
	ssCenter   = 0x00000001
	ssNotify   = 0x00000100
	ssBitmap   = 0x0000000E
	ssEditCtrl = 0x00002000
	swShow     = 5
	swRestore  = 9

	wmDestroy        = 0x0002
	wmClose          = 0x0010
	wmEraseBkgnd     = 0x0014
	wmSetCursor      = 0x0020
	wmSetFont        = 0x0030
	wmCommand        = 0x0111
	wmCtlColorStatic = 0x0138
	wmSetImage       = 0x0172

	imageBitmap   = 0
	smCxScreen    = 0
	smCyScreen    = 1
	bkTransparent = 1
	idcArrow      = 32512
	idcHand       = 32649

	// Dark title bar, Windows 10 20H1+. Failing silently on older builds is fine.
	dwmwaUseImmersiveDarkMode = 20
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procIsWindow            = user32.NewProc("IsWindow")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procSetCursor           = user32.NewProc("SetCursor")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procFillRect            = user32.NewProc("FillRect")
	procAdjustWindowRect    = user32.NewProc("AdjustWindowRect")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")

	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

	aboutClass = windows.StringToUTF16Ptr("KrypticAbout")
	wndProcCB  = windows.NewCallback(aboutWndProc)

	aboutMu   sync.Mutex
	aboutHWND windows.Handle

	activeTheme theme
	bgBrush     windows.Handle
	logoBMP     windows.Handle
	handCursor  uintptr

	titleFont windows.Handle
	bodyFont  windows.Handle
	smallFont windows.Handle
	linkFont  windows.Handle

	tagHWND   windows.Handle
	verHWND   windows.Handle
	blurbHWND windows.Handle
	linkHWND  windows.Handle
)

// theme mirrors the macOS About panel: primary/secondary/tertiary text plus a
// link accent, on a plain window background. Colors are COLORREF (0x00BBGGRR).
type theme struct {
	dark      bool
	bg        uint32
	primary   uint32
	secondary uint32
	tertiary  uint32
	link      uint32
}

func currentTheme() theme {
	if appsUseLightTheme() {
		return theme{
			dark:      false,
			bg:        0x00FFFFFF,
			primary:   0x001A1A1A,
			secondary: 0x0080726B, // #6b7280
			tertiary:  0x00AFA39C, // #9ca3af
			link:      0x00C06700, // #0067C0, Windows accent blue
		}
	}
	return theme{
		dark:      true,
		bg:        0x00202020,
		primary:   0x00F0F0F0,
		secondary: 0x00B8ADA6, // #a6adb8
		tertiary:  0x008E827A, // #7a828e
		link:      0x00FFC24C, // #4cc2ff, Windows dark-mode link blue
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

type wndClassEx struct {
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

type msg struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type rect struct {
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

func show() {
	runtime.LockOSThread()

	aboutMu.Lock()
	existing := aboutHWND
	aboutMu.Unlock()
	if existing != 0 {
		alive, _, _ := procIsWindow.Call(uintptr(existing))
		if alive != 0 {
			procShowWindow.Call(uintptr(existing), swRestore)
			procSetForegroundWindow.Call(uintptr(existing))
			return
		}
	}

	activeTheme = currentTheme()

	instance, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	handCursor, _, _ = procLoadCursorW.Call(0, idcHand)

	// Theme may have flipped since the last open: rebuild the brush and the
	// logo (which is composited against the background color).
	if bgBrush != 0 {
		procDeleteObject.Call(uintptr(bgBrush))
	}
	brush, _, _ := procCreateSolidBrush.Call(uintptr(activeTheme.bg))
	bgBrush = windows.Handle(brush)

	if logoBMP != 0 {
		procDeleteObject.Call(uintptr(logoBMP))
		logoBMP = 0
	}
	if bmp, err := hbitmapFromPNG(logoPNG, logoSize, logoSize, activeTheme.bg); err == nil {
		logoBMP = bmp
	}

	small, big := winui.AppIcons()
	class := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   wndProcCB,
		Instance:  windows.Handle(instance),
		Cursor:    windows.Handle(cursor),
		Icon:      big,
		IconSm:    small,
		ClassName: aboutClass,
		// No class background brush: wmEraseBkgnd paints with the live theme.
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))

	ensureFonts()

	style := uintptr(wsCaption | wsSysMenu)
	frame := rect{0, 0, aboutWidth, contentHeight()}
	procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&frame)), style, 0)
	winW := frame.Right - frame.Left
	winH := frame.Bottom - frame.Top

	screenW, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCyScreen)
	x := (int32(screenW) - winW) / 2
	y := (int32(screenH) - winH) / 2

	title, _ := windows.UTF16PtrFromString(WindowTitle)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(aboutClass)),
		uintptr(unsafe.Pointer(title)),
		style|wsVisible,
		uintptr(x), uintptr(y), uintptr(winW), uintptr(winH),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return
	}

	darkTitleBar := int32(0)
	if activeTheme.dark {
		darkTitleBar = 1
	}
	procDwmSetWindowAttribute.Call(hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&darkTitleBar)), 4)
	winui.ApplyChrome(windows.Handle(hwnd), activeTheme.dark)

	aboutMu.Lock()
	aboutHWND = windows.Handle(hwnd)
	aboutMu.Unlock()

	createChildren(windows.Handle(hwnd), windows.Handle(instance))
	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	aboutMu.Lock()
	aboutHWND = 0
	aboutMu.Unlock()
}

// contentHeight is the client height implied by createChildren's layout.
func contentHeight() int32 {
	h := int32(28) // top margin
	if logoBMP != 0 {
		h += logoSize + 18
	}
	h += 34 // app name
	h += 22 // tagline
	h += 30 // version
	h += 58 // blurb
	h += 20 // link
	h += 28 // bottom margin
	return h
}

func createChildren(parent, instance windows.Handle) {
	y := int32(28)
	if logoBMP != 0 {
		x := int32((aboutWidth - logoSize) / 2)
		hwnd := createControl("STATIC", "", wsChild|wsVisible|ssBitmap, x, y, logoSize, logoSize, parent, instance, 0)
		procSendMessageW.Call(uintptr(hwnd), wmSetImage, imageBitmap, uintptr(logoBMP))
		y += logoSize + 18
	}

	name := createControl("STATIC", AppName, wsChild|wsVisible|ssCenter, 30, y, 320, 30, parent, instance, 0)
	procSendMessageW.Call(uintptr(name), wmSetFont, uintptr(titleFont), 1)
	y += 34

	tagHWND = createControl("STATIC", Tagline, wsChild|wsVisible|ssCenter, 30, y, 320, 20, parent, instance, 0)
	procSendMessageW.Call(uintptr(tagHWND), wmSetFont, uintptr(bodyFont), 1)
	y += 22

	verHWND = createControl("STATIC", VersionLine(), wsChild|wsVisible|ssCenter, 30, y, 320, 18, parent, instance, 0)
	procSendMessageW.Call(uintptr(verHWND), wmSetFont, uintptr(smallFont), 1)
	y += 30

	blurbHWND = createControl("STATIC", Blurb, wsChild|wsVisible|ssCenter|ssEditCtrl, 30, y, 320, 48, parent, instance, 0)
	procSendMessageW.Call(uintptr(blurbHWND), wmSetFont, uintptr(bodyFont), 1)
	y += 58

	linkHWND = createControl("STATIC", WebsiteLabel, wsChild|wsVisible|ssCenter|ssNotify, 30, y, 320, 20, parent, instance, idLink)
	procSendMessageW.Call(uintptr(linkHWND), wmSetFont, uintptr(linkFont), 1)
}

func createControl(class, text string, style uint32, x, y, w, h int32, parent, instance windows.Handle, id uintptr) windows.Handle {
	classPtr, _ := windows.UTF16PtrFromString(class)
	textPtr, _ := windows.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), id, uintptr(instance), 0,
	)
	return windows.Handle(hwnd)
}

func aboutWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	switch message {
	case wmEraseBkgnd:
		var rc rect
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		procFillRect.Call(wparam, uintptr(unsafe.Pointer(&rc)), uintptr(bgBrush))
		return 1
	case wmCtlColorStatic:
		procSetBkMode.Call(wparam, bkTransparent)
		procSetTextColor.Call(wparam, uintptr(staticTextColor(windows.Handle(lparam))))
		return uintptr(bgBrush)
	case wmSetCursor:
		if linkHWND != 0 && windows.Handle(wparam) == linkHWND {
			procSetCursor.Call(handCursor)
			return 1
		}
	case wmCommand:
		if wparam&0xffff == idLink {
			OpenWebsite()
			return 0
		}
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, message, wparam, lparam)
	return ret
}

func staticTextColor(control windows.Handle) uint32 {
	switch control {
	case tagHWND, blurbHWND:
		return activeTheme.secondary
	case verHWND:
		return activeTheme.tertiary
	case linkHWND:
		return activeTheme.link
	default:
		return activeTheme.primary
	}
}

func ensureFonts() {
	if titleFont == 0 {
		titleFont = createFont(24, 600, false)
	}
	if bodyFont == 0 {
		bodyFont = createFont(14, 400, false)
	}
	if smallFont == 0 {
		smallFont = createFont(12, 400, false)
	}
	if linkFont == 0 {
		linkFont = createFont(14, 400, true)
	}
}

func createFont(px, weight int32, underline bool) windows.Handle {
	underlined := uintptr(0)
	if underline {
		underlined = 1
	}
	face, _ := windows.UTF16PtrFromString("Segoe UI")
	h, _, _ := procCreateFontW.Call(
		uintptr(px), 0, 0, 0, uintptr(weight), 0, underlined, 0,
		1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(face)),
	)
	return windows.Handle(h)
}

// hbitmapFromPNG composites the PNG over the window background color, because
// SS_BITMAP statics blit without alpha blending.
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
	hdc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdc)

	var bits unsafe.Pointer
	bmp, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&header)),
		0,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if bmp == 0 || bits == nil {
		return 0, syscall.EINVAL
	}

	// COLORREF is 0x00BBGGRR.
	bgR := int(byte(bg))
	bgG := int(byte(bg >> 8))
	bgB := int(byte(bg >> 16))

	dst := unsafe.Slice((*byte)(bits), w*h*4)
	for i := 0; i < w*h; i++ {
		r := int(scaled.Pix[i*4+0])
		g := int(scaled.Pix[i*4+1])
		b := int(scaled.Pix[i*4+2])
		a := int(scaled.Pix[i*4+3])
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
