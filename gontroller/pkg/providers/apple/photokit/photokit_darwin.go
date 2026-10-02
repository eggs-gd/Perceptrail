//go:build darwin

package photokit

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework AVFoundation -framework Photos
#include <stdlib.h>
#include "photokit.h"
*/
import "C"

import (
	"errors"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// main runs on the main thread (RunMain turns its run loop): Live Photo results are
// delivered on the main queue
func init() { runtime.LockOSThread() }

const authorized, limited = 3, 4

var allowed atomic.Bool

// Authorize asks for read access to Photos once (the system prompt names the app
// that started the process — the terminal, when run from one); false: no access,
// every request fails with ErrUnavailable
func Authorize() bool {
	st := int(C.pk_status())
	if st == 0 { // not determined yet
		st = int(C.pk_authorize())
	}
	allowed.Store(st == authorized || st == limited)
	return allowed.Load()
}

// RunMain serves the main queue until done; call it from main, in place of waiting
func RunMain(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		default:
			C.pk_run(0.1)
		}
	}
}

func image(uuid string, size int) ([]byte, error) {
	var buf unsafe.Pointer
	var n C.long
	err := call(uuid, func(u *C.char) *C.char { return C.pk_image(u, C.int(size), &buf, &n) })
	if buf == nil {
		return nil, err
	}
	defer C.free(buf)
	return C.GoBytes(buf, C.int(n)), err
}

func video(uuid string, mode int) (string, error) {
	var path *C.char
	err := call(uuid, func(u *C.char) *C.char { return C.pk_video(u, C.int(mode), &path) })
	return take(path), err
}

// take: a C string the caller frees, as Go ("" for NULL)
func take(s *C.char) string {
	if s == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(s))
	return C.GoString(s)
}

func live(uuid string) error {
	return call(uuid, func(u *C.char) *C.char { return C.pk_live(u) })
}

func call(uuid string, f func(*C.char) *C.char) error {
	if !allowed.Load() {
		return ErrUnavailable
	}
	u := C.CString(uuid)
	defer C.free(unsafe.Pointer(u))
	if e := f(u); e != nil {
		defer C.free(unsafe.Pointer(e))
		return errors.New("photokit: " + C.GoString(e))
	}
	return nil
}
