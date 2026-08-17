//go:build windows

package authstore

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Credential Manager via advapi32 - the Windows equivalent of the
// macOS Keychain item. Stored as a generic credential visible only to the
// current user.
const credentialTarget = "dev.kryptic.daemon/refresh-token"

const credTypeGeneric = 1       // CRED_TYPE_GENERIC
const credPersistLocalMachine = 2 // CRED_PERSIST_LOCAL_MACHINE

var (
	advapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

// credential mirrors CREDENTIALW (minus the fields we never set).
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func platformSave(refreshToken string) error {
	targetName, err := windows.UTF16PtrFromString(credentialTarget)
	if err != nil {
		return err
	}
	userName, err := windows.UTF16PtrFromString("kryptic")
	if err != nil {
		return err
	}

	blob := []byte(refreshToken)
	cred := credential{
		Type:               credTypeGeneric,
		TargetName:         targetName,
		CredentialBlobSize: uint32(len(blob)),
		Persist:            credPersistLocalMachine,
		UserName:           userName,
	}
	if len(blob) > 0 {
		cred.CredentialBlob = &blob[0]
	}

	result, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if result == 0 {
		return errors.New("credential manager write failed: " + callErr.Error())
	}
	return nil
}

func platformLoad() (string, error) {
	targetName, err := windows.UTF16PtrFromString(credentialTarget)
	if err != nil {
		return "", err
	}

	var credPtr *credential
	result, _, _ := procCredRead.Call(
		uintptr(unsafe.Pointer(targetName)), credTypeGeneric, 0,
		uintptr(unsafe.Pointer(&credPtr)))
	if result == 0 || credPtr == nil {
		return "", ErrNotLoggedIn
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(credPtr)))

	if credPtr.CredentialBlobSize == 0 || credPtr.CredentialBlob == nil {
		return "", ErrNotLoggedIn
	}
	blob := unsafe.Slice(credPtr.CredentialBlob, credPtr.CredentialBlobSize)
	token := make([]byte, len(blob))
	copy(token, blob)
	return string(token), nil
}

func platformClear() {
	targetName, err := windows.UTF16PtrFromString(credentialTarget)
	if err != nil {
		return
	}
	_, _, _ = procCredDelete.Call(uintptr(unsafe.Pointer(targetName)), credTypeGeneric, 0)
}
