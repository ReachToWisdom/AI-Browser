package main

import (
	"syscall"
	"unsafe"
)

// webview 구조체 레이아웃 (jchv/go-webview2 기준)
// hwnd(8) + mainthread(8) + browser(interface=16) + ...
type webviewLayout struct {
	hwnd        uintptr // offset 0
	mainthread  uintptr // offset 8
	browserType uintptr // offset 16: interface type ptr
	browserData uintptr // offset 24: interface data ptr → *edge.Chromium
}

// Chromium 구조체 레이아웃 (pkg/edge 기준)
// hwnd(8) + focusOnInit(bool=1 + pad=7 → 8) + controller(8) + ...
type chromiumLayout struct {
	hwnd        uintptr  // offset 0
	focusOnInit [8]byte  // offset 8 (bool + padding)
	controller  uintptr  // offset 16: *ICoreWebView2Controller
}

// vtable 오프셋 (IUnknown(3) + ...)
const putIsVisibleVtblOffset = 4 * unsafe.Sizeof(uintptr(0)) // PutIsVisible
const putBoundsVtblOffset = 6 * unsafe.Sizeof(uintptr(0))    // PutBounds

// WebView2의 PutBounds를 직접 호출
func putWebViewBounds(parentHWND uintptr, bounds RECT) bool {
	if webviewInstance == nil {
		return false
	}

	// Go 인터페이스에서 concrete 포인터 추출
	type iface struct {
		tab  uintptr
		data uintptr
	}
	wvIface := *(*iface)(unsafe.Pointer(&webviewInstance))
	if wvIface.data == 0 {
		return false
	}

	// webview → browser(interface) → Chromium → controller
	wvLayout := (*webviewLayout)(unsafe.Pointer(wvIface.data))
	chromiumPtr := wvLayout.browserData
	if chromiumPtr == 0 {
		return false
	}

	cLayout := (*chromiumLayout)(unsafe.Pointer(chromiumPtr))
	controllerPtr := cLayout.controller
	if controllerPtr == 0 {
		return false
	}

	// vtable → PutBounds 함수 포인터
	vtblPtr := *(*uintptr)(unsafe.Pointer(controllerPtr))
	if vtblPtr == 0 {
		return false
	}
	putBoundsFn := *(*uintptr)(unsafe.Pointer(vtblPtr + uintptr(putBoundsVtblOffset)))
	if putBoundsFn == 0 {
		return false
	}

	// COM 호출: PutBounds(this, *RECT)
	syscall.SyscallN(putBoundsFn, controllerPtr, uintptr(unsafe.Pointer(&bounds)))
	return true
}

// 메인 웹뷰의 controller 포인터 추출 (공통 헬퍼)
func getMainControllerPtr() uintptr {
	if webviewInstance == nil {
		return 0
	}
	type iface struct {
		tab  uintptr
		data uintptr
	}
	wvIface := *(*iface)(unsafe.Pointer(&webviewInstance))
	if wvIface.data == 0 {
		return 0
	}
	wvLayout := (*webviewLayout)(unsafe.Pointer(wvIface.data))
	if wvLayout.browserData == 0 {
		return 0
	}
	cLayout := (*chromiumLayout)(unsafe.Pointer(wvLayout.browserData))
	return cLayout.controller
}

// 메인 웹뷰 표시/숨김 (ICoreWebView2Controller.PutIsVisible)
func putMainWebViewVisible(visible bool) bool {
	controllerPtr := getMainControllerPtr()
	if controllerPtr == 0 {
		return false
	}
	vtblPtr := *(*uintptr)(unsafe.Pointer(controllerPtr))
	if vtblPtr == 0 {
		return false
	}
	fn := *(*uintptr)(unsafe.Pointer(vtblPtr + uintptr(putIsVisibleVtblOffset)))
	if fn == 0 {
		return false
	}
	v := uintptr(0)
	if visible {
		v = 1
	}
	syscall.SyscallN(fn, controllerPtr, v)
	return true
}

// 메인 웹뷰 리사이즈 (설정 페이지 표시 시 사용)
func resizeMainWebView() {
	if mainHWND == 0 {
		return
	}
	var rect RECT
	pGetClientRect.Call(mainHWND, uintptr(unsafe.Pointer(&rect)))
	bounds := RECT{0, int32(TABBAR_H), rect.Right, rect.Bottom}
	putWebViewBounds(mainHWND, bounds)
}
