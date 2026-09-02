//go:build windows

package dialog

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
)

var (
	clsidFileOpenDialog = windows.GUID{
		Data1: 0xDC1C5A9C,
		Data2: 0xE88A,
		Data3: 0x4DDE,
		Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7},
	}
	iidIFileOpenDialog = windows.GUID{
		Data1: 0xD57C7288,
		Data2: 0xD4AD,
		Data3: 0x4768,
		Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60},
	}
)

const (
	coinitApartmentThreaded = 0x2
	clsctxInprocServer      = 1
	fosPickFolders          = 0x20
	fosForceFileSystem      = 0x40
	sigdnFileSysPath        = 0x80058000
	sFalse                  = 1
)

// PickFolder shows the native directory picker. ok is false if cancelled.
func PickFolder(title string) (string, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if hr == 0 || hr == sFalse {
		defer procCoUninitialize.Call()
	}

	var dlg uintptr
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)),
		uintptr(unsafe.Pointer(&dlg)),
	)
	if hr != 0 || dlg == 0 {
		return "", false
	}
	defer comCall(dlg, 2) // Release

	var options uint32
	if comCall(dlg, 10, uintptr(unsafe.Pointer(&options))) == 0 { // GetOptions
		_ = comCall(dlg, 9, uintptr(options|fosPickFolders|fosForceFileSystem)) // SetOptions
	}
	if title != "" {
		titlePtr, err := windows.UTF16PtrFromString(title)
		if err == nil {
			_ = comCall(dlg, 17, uintptr(unsafe.Pointer(titlePtr))) // SetTitle
		}
	}

	if comCall(dlg, 3, 0) != 0 { // Show
		return "", false
	}

	var item uintptr
	if comCall(dlg, 20, uintptr(unsafe.Pointer(&item))) != 0 || item == 0 { // GetResult
		return "", false
	}
	defer comCall(item, 2)

	var name uintptr
	if comCall(item, 5, sigdnFileSysPath, uintptr(unsafe.Pointer(&name))) != 0 || name == 0 {
		return "", false
	}
	defer procCoTaskMemFree.Call(name)

	path := windows.UTF16PtrToString((*uint16)(unsafe.Pointer(name)))
	return path, path != ""
}

func comCall(obj uintptr, method uintptr, args ...uintptr) uintptr {
	if obj == 0 {
		return 0x80004003
	}
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + method*unsafe.Sizeof(uintptr(0))))
	callArgs := make([]uintptr, 0, 1+len(args))
	callArgs = append(callArgs, obj)
	callArgs = append(callArgs, args...)
	r, _, _ := syscall.SyscallN(fn, callArgs...)
	return r
}
