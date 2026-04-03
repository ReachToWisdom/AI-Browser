// webview2_events.go - 팝업 → 현재 탭 내비게이션, 프로세스 크래시 → 자동 복구
package main

import (
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
)

// ICoreWebView2 vtable 오프셋 (IUnknown 3개 포함)
const (
	wvVtblAddProcessFailed      = 25
	wvVtblAddNewWindowRequested = 44
)

// NewWindowRequestedEventArgs vtable 오프셋
const (
	nwArgsGetUri     = 3
	nwArgsPutHandled = 7
)

// ProcessFailedEventArgs vtable 오프셋
const (
	pfArgsGetKind = 3
)

// 프로세스 실패 종류
const (
	pfKindBrowserExited      = 0
	pfKindRenderExited       = 1
	pfKindRenderUnresponsive = 2
)

// COM 이벤트 핸들러 (IUnknown + Invoke)
type comHandler struct {
	vtbl *comHandlerVtbl
	fn   func(sender, args uintptr) uintptr
}

type comHandlerVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Invoke         uintptr
}

// 공유 vtable 콜백 (전역 1세트, 모든 핸들러가 공유)
var handlerVtblShared = &comHandlerVtbl{
	QueryInterface: syscall.NewCallback(func(this, refiid, obj uintptr) uintptr {
		*(*uintptr)(unsafe.Pointer(obj)) = this
		return 0
	}),
	AddRef:  syscall.NewCallback(func(this uintptr) uintptr { return 1 }),
	Release: syscall.NewCallback(func(this uintptr) uintptr { return 1 }),
	Invoke: syscall.NewCallback(func(this, sender, args uintptr) uintptr {
		h := (*comHandler)(unsafe.Pointer(this))
		if h.fn != nil {
			return h.fn(sender, args)
		}
		return 0
	}),
}

// GC 방지용 참조
var handlerStore []*comHandler

func newComHandler(fn func(sender, args uintptr) uintptr) *comHandler {
	h := &comHandler{vtbl: handlerVtblShared, fn: fn}
	handlerStore = append(handlerStore, h)
	return h
}

// ole32 CoTaskMemFree (COM 할당 메모리 해제)
var ole32            = syscall.NewLazyDLL("ole32.dll")
var pCoTaskMemFree   = ole32.NewProc("CoTaskMemFree")

// UTF16 포인터 → Go string 변환
func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	buf := make([]uint16, 0, 256)
	for ptr := uintptr(unsafe.Pointer(p)); ; ptr += 2 {
		ch := *(*uint16)(unsafe.Pointer(ptr))
		if ch == 0 {
			break
		}
		buf = append(buf, ch)
	}
	return syscall.UTF16ToString(buf)
}

// Chromium 내부 webview(ICoreWebView2) 포인터 (offset 24)
func getWebView(c *edge.Chromium) uintptr {
	type layout struct {
		_hwnd   uintptr
		_focus  [8]byte
		_ctrl   uintptr
		webview uintptr
	}
	return (*layout)(unsafe.Pointer(c)).webview
}

// COM vtable 함수 포인터 읽기
func comFn(obj uintptr, idx int) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	return *(*uintptr)(unsafe.Pointer(vtbl + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
}

// 탭 WebView2에 이벤트 핸들러 등록
func hookTabEvents(c *edge.Chromium) {
	wv := getWebView(c)
	if wv == 0 {
		return
	}

	var token int64

	// 1) NewWindowRequested → 시스템 기본 브라우저로 열기 (팝업/메일/쪽지 등)
	nwHandler := newComHandler(func(sender, args uintptr) uintptr {
		var uriPtr *uint16
		syscall.SyscallN(comFn(args, nwArgsGetUri),
			args, uintptr(unsafe.Pointer(&uriPtr)))
		if uriPtr != nil {
			popupURL := utf16PtrToString(uriPtr)
			pCoTaskMemFree.Call(uintptr(unsafe.Pointer(uriPtr)))
			if popupURL != "" && popupURL != "about:blank" {
				openURLInBrowser(popupURL)
			}
		}
		// PutHandled(true) → WebView2 내부에서 새 창 열지 않음
		syscall.SyscallN(comFn(args, nwArgsPutHandled), args, 1)
		return 0
	})
	syscall.SyscallN(comFn(wv, wvVtblAddNewWindowRequested),
		wv, uintptr(unsafe.Pointer(nwHandler)), uintptr(unsafe.Pointer(&token)))

	// 2) ProcessFailed → 자동 복구
	pfHandler := newComHandler(func(sender, args uintptr) uintptr {
		var kind uint32
		syscall.SyscallN(comFn(args, pfArgsGetKind),
			args, uintptr(unsafe.Pointer(&kind)))

		// 해당 탭 찾기
		tabMutex.Lock()
		idx := -1
		for i, tc := range tabChromiums {
			if tc == c {
				idx = i
				break
			}
		}
		var targetURL string
		if idx >= 0 && idx < len(allTabs) {
			targetURL = allTabs[idx].URL
		}
		tabMutex.Unlock()

		if idx < 0 {
			return 0
		}

		switch kind {
		case pfKindRenderExited, pfKindRenderUnresponsive:
			// 렌더러 크래시 → 원래 URL로 재이동
			if targetURL != "" {
				c.Navigate(targetURL)
			}
		case pfKindBrowserExited:
			// 브라우저 프로세스 종료 → Chromium 재생성
			if webviewInstance != nil {
				tabIdx := idx
				webviewInstance.Dispatch(func() {
					recreateTab(tabIdx)
				})
			}
		}
		return 0
	})
	syscall.SyscallN(comFn(wv, wvVtblAddProcessFailed),
		wv, uintptr(unsafe.Pointer(pfHandler)), uintptr(unsafe.Pointer(&token)))
}

// 탭 Chromium 재생성 (브라우저 프로세스 완전 종료 시)
func recreateTab(idx int) {
	tabMutex.Lock()
	if idx < 0 || idx >= len(tabChromiums) {
		tabMutex.Unlock()
		return
	}
	// 기존 Chromium 참조 제거
	tabChromiums[idx] = nil
	tabMutex.Unlock()

	// 새 Chromium 생성
	c := ensureTabChromium(idx)
	if c != nil && idx == activeTab {
		setChromiumBounds(c)
		c.Show()
		c.Focus()
	}
}
