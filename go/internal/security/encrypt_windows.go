//go:build windows

package security

import (
	"syscall"
	"unsafe"
)

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

type _DATA_BLOB struct {
	cbData uint32
	pbData *byte
}

func encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	in := _DATA_BLOB{cbData: uint32(len(plaintext)), pbData: &plaintext[0]}
	var out _DATA_BLOB
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return unsafe.Slice(out.pbData, out.cbData), nil
}

func decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	in := _DATA_BLOB{cbData: uint32(len(ciphertext)), pbData: &ciphertext[0]}
	var out _DATA_BLOB
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return unsafe.Slice(out.pbData, out.cbData), nil
}
