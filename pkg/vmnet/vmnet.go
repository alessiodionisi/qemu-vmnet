package vmnet

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -lobjc -framework vmnet
// #include "vmnet.h"
import "C"
import (
	"io"
	"unsafe"
)

type VMNet struct {
	io.ReadWriter

	iface         C.interface_ref
	mps           C.ulonglong
	MaxPacketSize int
}

func (v *VMNet) Start() error {
	C._vmnet_start(&v.iface, &v.mps)
	if v.iface == nil {
		return ErrUnableToStart
	}

	v.MaxPacketSize = int(v.mps)

	return nil
}

func (v *VMNet) Stop() error {
	C._vmnet_stop(v.iface)
	return nil
}

func (v *VMNet) Read(p []byte) (n int, err error) {
	for {
		var cBytes unsafe.Pointer
		var cBytesLen C.ulong

		C._vmnet_read(v.iface, v.mps, &cBytes, &cBytesLen)

		if cBytes == nil {
			continue
		}

		copy(p, C.GoBytes(cBytes, C.int(cBytesLen)))
		return int(cBytesLen), nil
	}
}

func (v *VMNet) Write(p []byte) (n int, err error) {
	C._vmnet_write(v.iface, C.CBytes(p), C.ulong(len(p)))
	return len(p), nil
}

func New() *VMNet {
	return &VMNet{}
}
