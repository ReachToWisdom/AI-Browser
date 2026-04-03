package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

var (
	pCreateMutexW    = kernel32.NewProc("CreateMutexW")
	pGetLastError    = kernel32.NewProc("GetLastError")
	pGetClientRect   = user32.NewProc("GetClientRect")
	pFindWindowExW   = user32.NewProc("FindWindowExW")
	pFindWindowW     = user32.NewProc("FindWindowW")
	pMoveWindow      = user32.NewProc("MoveWindow")
	pInvalidateRect  = user32.NewProc("InvalidateRect")
	pIsWindowVisible = user32.NewProc("IsWindowVisible")
	pGetWindowRect   = user32.NewProc("GetWindowRect")
)

const ERROR_ALREADY_EXISTS = 183

// 전역 WebView 참조 (설정 페이지 전용)
var webviewInstance webview2.WebView

func main() {
	runtime.LockOSThread()

	// 디버그 로그 파일 설정
	logFile, err := os.OpenFile(filepath.Join(getAppDir(), "crash.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	// AppUserModelID 설정 (작업표시줄 아이콘이 Electron/Edge로 바뀌는 문제 방지)
	setAppUserModelID("HyeTong.AIBrowser")

	// 단일 인스턴스 보장 (Mutex)
	mutexName := syscall.StringToUTF16Ptr("Global\\AIBrowserMutex")
	_, _, err = pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	if err == syscall.Errno(ERROR_ALREADY_EXISTS) {
		bringExistingWindow()
		os.Exit(0)
	}

	// 설정 로드
	loadTabs()
	loadState()

	// 저장된 활성 탭 복원
	if appState.ActiveTab >= 0 && appState.ActiveTab < len(allTabs) {
		activeTab = appState.ActiveTab
	}

	// WebView2 생성 (설정 페이지 전용 + 메시지 루프 호스트)
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  APP_NAME,
			Width:  uint(WINDOW_W),
			Height: uint(WINDOW_H),
			IconId: IDI_APPICON,
			Center: true,
		},
	})
	if w == nil {
		fmt.Println("WebView2를 초기화할 수 없습니다.")
		fmt.Println("Microsoft Edge WebView2 Runtime이 설치되어 있는지 확인하세요.")
		os.Exit(1)
	}
	defer w.Destroy()
	webviewInstance = w

	// 탭 전환 콜백 (트레이/탭바에서 호출)
	navigateCallback = func(idx int) {
		w.Dispatch(func() {
			switchToTab(idx)
		})
	}

	// Go 함수 바인딩 (설정 페이지에서 사용)
	w.Bind("removeTab", func(idx int) {
		if removeTabAt(idx) {
			refreshTabBar()
		}
	})
	w.Bind("addNewTab", func(name, url string, color uint32) {
		addTabWithColor(name, url, color)
		refreshTabBar()
	})
	w.Bind("reorderTabs", func(fromIdx, toIdx int) {
		reorderTab(fromIdx, toIdx)
		refreshTabBar()
	})
	w.Bind("goBackToTab", func() {
		closeSettingsView()
	})

	// 메인 웹뷰는 빈 페이지 (설정 시에만 사용)
	w.Navigate("about:blank")

	// 창 준비 후 초기화
	w.Dispatch(func() {
		hwnd := uintptr(w.Window())
		if hwnd != 0 {
			mainHWND = hwnd

			// 저장된 창 위치/크기 복원
			if appState.WindowW > 0 && appState.WindowH > 0 {
				pMoveWindow.Call(hwnd,
					uintptr(appState.WindowX), uintptr(appState.WindowY),
					uintptr(appState.WindowW), uintptr(appState.WindowH), 1)
			}

			// 네이티브 탭바 생성
			createTabBar(hwnd)
			// WndProc 서브클래싱
			subclassWindow(hwnd)
			// 초기 레이아웃
			layoutChildren(hwnd)

			// 메인 웹뷰 숨김 (설정용으로만 표시)
			putMainWebViewVisible(false)

			// 탭 뷰 초기화 + 첫 탭 표시
			initTabViews()
			if c := ensureTabChromium(activeTab); c != nil {
				setChromiumBounds(c)
				c.Show()
				c.Focus()
			}

			// 캐시 정리 (백그라운드)
			go func() {
				time.Sleep(5 * time.Second)
				runCleanup()
			}()

			// 업데이트 확인 (백그라운드)
			go startUpdateChecker()

			// 시스템 트레이
			go startTray(hwnd, func() {
				saveWindowState()
				saveState()
				removeTray()
				os.Exit(0)
			})
		}
	})

	w.Run()
	saveWindowState()
	saveState()
	removeTray()
}

// 현재 창 위치/크기를 appState에 저장
func saveWindowState() {
	if mainHWND == 0 {
		return
	}
	var rect RECT
	ret, _, _ := pGetWindowRect.Call(mainHWND, uintptr(unsafe.Pointer(&rect)))
	if ret != 0 {
		appState.WindowX = int(rect.Left)
		appState.WindowY = int(rect.Top)
		appState.WindowW = int(rect.Right - rect.Left)
		appState.WindowH = int(rect.Bottom - rect.Top)
	}
}

// 중복 실행 시 기존 창을 찾아 복원
func bringExistingWindow() {
	hwnd, _, _ := pFindWindowW.Call(0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(APP_NAME))))
	if hwnd != 0 {
		pShowWindow.Call(hwnd, SW_SHOW)
		pShowWindow.Call(hwnd, SW_RESTORE)
		pSetForegroundWindow.Call(hwnd)
	}
}

const WM_CLOSE = 0x0010
const WM_SIZE = 0x0005
const GWLP_WNDPROC = ^uintptr(3)

type RECT struct {
	Left, Top, Right, Bottom int32
}

func subclassWindow(hwnd uintptr) {
	origWndProc, _, _ = pGetWindowLongPtrW.Call(hwnd, GWLP_WNDPROC)
	if origWndProc == 0 {
		return
	}

	newProc := syscall.NewCallback(func(h syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
		switch msg {
		case WM_CLOSE:
			hideMainWindow()
			return 0
		case WM_SIZE:
			ret, _, _ := pCallWindowProcW.Call(origWndProc, uintptr(h), uintptr(msg), wParam, lParam)
			layoutChildren(uintptr(h))
			return ret
		}
		ret, _, _ := pCallWindowProcW.Call(origWndProc, uintptr(h), uintptr(msg), wParam, lParam)
		return ret
	})

	pSetWindowLongPtrW.Call(hwnd, GWLP_WNDPROC, newProc)
}

// WebView2와 탭바의 레이아웃 조정
func layoutChildren(parentHWND uintptr) {
	var rect RECT
	pGetClientRect.Call(parentHWND, uintptr(unsafe.Pointer(&rect)))
	w := int(rect.Right)
	h := int(rect.Bottom)

	// 탭바 위치 (상단)
	if tabBarHWND != 0 {
		pMoveWindow.Call(uintptr(tabBarHWND), 0, 0, uintptr(w), uintptr(TABBAR_H), 1)
	}

	// 메인 웹뷰 bounds (설정 페이지용)
	bounds := RECT{
		Left:   0,
		Top:    int32(TABBAR_H),
		Right:  int32(w),
		Bottom: int32(h),
	}
	putWebViewBounds(parentHWND, bounds)

	// 모든 탭 WebView2 리사이즈
	resizeSecondaryTabs()
}
