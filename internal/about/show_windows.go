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

	"golang.org/x/sys/windows"
)

const (
	aboutWidth  = 380
	aboutHeight = 400
	idLink      = 1001

	wsVisible   = 0x10000000
	wsCaption   = 0x00C00000
	wsSysMenu   = 0x00080000
	wsChild     = 0x40000000
	wsTabStop   = 0x00010000
	ssCenter    = 0x00000001
	ssBitmap    = 0x0000000E
	ssEditCtrl  = 0x00002000
	swShow      = 5
	swRestore   = 9
	wmDestroy   = 0x0002
	wmClose     = 0x0010
	wmCommand   = 0x0111
	wmSetFont   = 0x0030
	wmSetImage  = 0x0172
	imageBitmap = 0
	smCxScreen  = 0
	smCyScreen  = 1
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

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
	procSendMessageW        = user32.NewProc("SendMessageW")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")

	aboutClass = windows.StringToUTF16Ptr("KrypticAbout")
	wndProcCB  = windows.NewCallback(aboutWndProc)

	aboutMu   sync.Mutex
	aboutHWND windows.Handle
	logoBMP   windows.Handle
	titleFont windows.Handle
	bodyFont  windows.Handle
	smallFont windows.Handle
)

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

	instance, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, 32512) // IDC_ARROW
	bg, _, _ := procCreateSolidBrush.Call(0x00FFFFFF)

	class := wndClassEx{
		Size:       uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:    wndProcCB,
		Instance:   windows.Handle(instance),
		Cursor:     windows.Handle(cursor),
		Background: windows.Handle(bg),
		ClassName:  aboutClass,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))

	ensureFonts()
	if logoBMP == 0 {
		if bmp, err := hbitmapFromPNG(logoPNG, 220, 80); err == nil {
			logoBMP = bmp
		}
	}

	screenW, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCyScreen)
	x := (int32(screenW) - aboutWidth) / 2
	y := (int32(screenH) - aboutHeight) / 2

	title, _ := windows.UTF16PtrFromString(WindowTitle)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(aboutClass)),
		uintptr(unsafe.Pointer(title)),
		wsCaption|wsSysMenu|wsVisible,
		uintptr(x), uintptr(y), aboutWidth, aboutHeight,
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return
	}

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

func createChildren(parent, instance windows.Handle) {
	y := int32(24)
	if logoBMP != 0 {
		logoH := int32(80)
		hwnd := createControl("STATIC", "", wsChild|wsVisible|ssBitmap, 80, y, 220, logoH, parent, instance, 0)
		procSendMessageW.Call(uintptr(hwnd), wmSetImage, imageBitmap, uintptr(logoBMP))
		y += logoH + 16
	}

	name := createControl("STATIC", AppName, wsChild|wsVisible|ssCenter, 30, y, 320, 28, parent, instance, 0)
	procSendMessageW.Call(uintptr(name), wmSetFont, uintptr(titleFont), 1)
	y += 32

	tag := createControl("STATIC", Tagline, wsChild|wsVisible|ssCenter, 30, y, 320, 20, parent, instance, 0)
	procSendMessageW.Call(uintptr(tag), wmSetFont, uintptr(bodyFont), 1)
	y += 22

	ver := createControl("STATIC", VersionLine(), wsChild|wsVisible|ssCenter, 30, y, 320, 18, parent, instance, 0)
	procSendMessageW.Call(uintptr(ver), wmSetFont, uintptr(smallFont), 1)
	y += 28

	blurb := createControl("STATIC", Blurb, wsChild|wsVisible|ssCenter|ssEditCtrl, 30, y, 320, 48, parent, instance, 0)
	procSendMessageW.Call(uintptr(blurb), wmSetFont, uintptr(bodyFont), 1)
	y += 56

	link := createControl("BUTTON", WebsiteLabel, wsChild|wsVisible|wsTabStop, 120, y, 140, 28, parent, instance, idLink)
	procSendMessageW.Call(uintptr(link), wmSetFont, uintptr(bodyFont), 1)
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

func aboutWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
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
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
}

func ensureFonts() {
	if titleFont == 0 {
		titleFont = createFont(22, 600)
	}
	if bodyFont == 0 {
		bodyFont = createFont(13, 400)
	}
	if smallFont == 0 {
		smallFont = createFont(11, 400)
	}
}

func createFont(px, weight int32) windows.Handle {
	face, _ := windows.UTF16PtrFromString("Segoe UI")
	h, _, _ := procCreateFontW.Call(
		uintptr(px), 0, 0, 0, uintptr(weight), 0, 0, 0,
		1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(face)),
	)
	return windows.Handle(h)
}

func hbitmapFromPNG(data []byte, maxW, maxH int) (windows.Handle, error) {
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

	dst := unsafe.Slice((*byte)(bits), w*h*4)
	for i := 0; i < w*h; i++ {
		r, g, b, a := scaled.Pix[i*4+0], scaled.Pix[i*4+1], scaled.Pix[i*4+2], scaled.Pix[i*4+3]
		dst[i*4+0] = b
		dst[i*4+1] = g
		dst[i*4+2] = r
		dst[i*4+3] = a
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
