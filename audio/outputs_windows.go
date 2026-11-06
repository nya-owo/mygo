//go:build windows && (amd64 || arm64)

package audio

import (
	"runtime"
	"syscall"
	"unsafe"
)

const (
	deviceStateActive = 1
	stgmRead          = 0
	vtLPWSTR          = 31
)

var (
	pkeyDeviceFriendlyName = propertyKey{guid{0xa45c254e, 0xdf1c, 0x4efd, [8]byte{0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}}, 14}
	procPropVariantClear   = ole32.NewProc("PropVariantClear")
	procCoUninitialize     = ole32.NewProc("CoUninitialize")
)

type propertyKey struct {
	fmtid guid
	pid   uint32
}

// propVariant is a PROPVARIANT: its type, then its value, a pointer for
// the strings asked for here.
type propVariant struct {
	vt       uint16
	_        [3]uint16
	val      uintptr
	_        uintptr
}

// listOutputs lists the active render endpoints, on a thread of its own in
// COM's multithreaded apartment, whatever the caller's thread is in.
func listOutputs() ([]Output, error) {
	type result struct {
		outs []Output
		err  error
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		hr, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
		if !failed(hr) {
			defer procCoUninitialize.Call()
		}
		outs, err := enumOutputs()
		done <- result{outs, err}
	}()
	r := <-done
	return r.outs, r.err
}

func enumOutputs() ([]Output, error) {
	var enumerator uintptr
	if hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enumerator))); failed(hr) {
		return nil, hresult("CoCreateInstance(MMDeviceEnumerator)", hr)
	}
	defer release(enumerator)
	def, _ := defaultID(enumerator)
	var coll uintptr
	if hr := comCall(enumerator, 3, eRender, deviceStateActive, uintptr(unsafe.Pointer(&coll))); failed(hr) {
		return nil, hresult("EnumAudioEndpoints", hr)
	}
	defer release(coll)
	var n uint32
	if hr := comCall(coll, 3, uintptr(unsafe.Pointer(&n))); failed(hr) {
		return nil, hresult("IMMDeviceCollection.GetCount", hr)
	}
	var outs []Output
	for i := range n {
		var dev uintptr
		if hr := comCall(coll, 4, uintptr(i), uintptr(unsafe.Pointer(&dev))); failed(hr) {
			continue
		}
		id := deviceID(dev)
		name := friendlyName(dev)
		release(dev)
		if id == "" {
			continue
		}
		if name == "" {
			name = id
		}
		outs = append(outs, Output{ID: id, Name: name, Default: id == def})
	}
	return outs, nil
}

// deviceID returns an IMMDevice's endpoint ID, or "".
func deviceID(dev uintptr) string {
	var p *uint16
	if hr := comCall(dev, 5, uintptr(unsafe.Pointer(&p))); failed(hr) || p == nil {
		return ""
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return utf16At(p)
}

// friendlyName returns what the system calls an IMMDevice, or "".
func friendlyName(dev uintptr) string {
	var store uintptr
	if hr := comCall(dev, 4, stgmRead, uintptr(unsafe.Pointer(&store))); failed(hr) {
		return ""
	}
	defer release(store)
	var v propVariant
	if hr := comCall(store, 5, uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&v))); failed(hr) {
		return ""
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&v)))
	if v.vt != vtLPWSTR || v.val == 0 {
		return ""
	}
	return utf16At((*uint16)(native(v.val)))
}

// utf16At reads the zero-ended UTF-16 string at p.
func utf16At(p *uint16) string {
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), n*2)) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// outputActive reports whether the output of id is there to play through.
func outputActive(enumerator uintptr, id string) bool {
	p, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return false
	}
	var dev uintptr
	if hr := comCall(enumerator, 5, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&dev))); failed(hr) {
		return false
	}
	defer release(dev)
	var state uint32
	return !failed(comCall(dev, 6, uintptr(unsafe.Pointer(&state)))) && state == deviceStateActive
}
