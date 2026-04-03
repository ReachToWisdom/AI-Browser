package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	pRegisterClassExW       = user32.NewProc("RegisterClassExW")
	pCreateWindowExW        = user32.NewProc("CreateWindowExW")
	pDefWindowProcW         = user32.NewProc("DefWindowProcW")
	pGetMessageW            = user32.NewProc("GetMessageW")
	pTranslateMessage       = user32.NewProc("TranslateMessage")
	pDispatchMessageW       = user32.NewProc("DispatchMessageW")
	pPostQuitMessage        = user32.NewProc("PostQuitMessage")
	pShell_NotifyIconW      = shell32.NewProc("Shell_NotifyIconW")
	pCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	pAppendMenuW            = user32.NewProc("AppendMenuW")
	pTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	pDestroyMenu            = user32.NewProc("DestroyMenu")
	pGetCursorPos           = user32.NewProc("GetCursorPos")
	pSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	pPostMessageW           = user32.NewProc("PostMessageW")
	pGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	pShowWindow             = user32.NewProc("ShowWindow")
	pSetWindowLongPtrW      = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW        = user32.NewProc("CallWindowProcW")
	pGetWindowLongPtrW      = user32.NewProc("GetWindowLongPtrW")
	pSetForegroundWindowProc = user32.NewProc("SetForegroundWindow")
	pLoadIconW              = user32.NewProc("LoadIconW")
	pLoadImageW             = user32.NewProc("LoadImageW")
	pMessageBoxW            = user32.NewProc("MessageBoxW")
)

const (
	IMAGE_ICON    = 1
	LR_LOADFROMFILE = 0x0010
	LR_DEFAULTSIZE  = 0x0040
)

const (
	NIM_ADD    = 0x00
	NIM_DELETE = 0x02
	NIF_ICON   = 0x02
	NIF_TIP    = 0x04
	NIF_MESSAGE = 0x01
	NIF_INFO   = 0x10

	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205
	WM_COMMAND       = 0x0111
	WM_DESTROY       = 0x0002
	WM_NULL          = 0x0000

	SW_HIDE    = 0
	SW_SHOW    = 5
	SW_RESTORE = 9

	MF_STRING    = 0x0000
	MF_SEPARATOR = 0x0800

	TPM_BOTTOMALIGN = 0x0020
	TPM_LEFTALIGN   = 0x0000

	GWL_WNDPROC = -4

	IDI_APPLICATION = 32512

	// 메뉴 ID
	MENU_SHOW  = 1000
	MENU_ABOUT = 1098
	MENU_QUIT  = 1099

	// MessageBox 플래그
	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
)

// NOTIFYICONDATAW 구조체
type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     syscall.Handle
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

type MSG struct {
	HWnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type POINT struct {
	X, Y int32
}

// 글로벌 참조
var (
	trayHWnd    syscall.Handle
	trayNID     NOTIFYICONDATAW
	mainHWND    uintptr // WebView2 창 핸들
	origWndProc uintptr // 원본 WndProc
	onQuit      func()  // 종료 콜백
)

// 시스템 트레이 시작 (별도 goroutine에서 실행)
func startTray(webviewHWND uintptr, quitFn func()) {
	mainHWND = webviewHWND
	onQuit = quitFn

	runtime.LockOSThread()

	hInst, _, _ := pGetModuleHandleW.Call(0)
	className := syscall.StringToUTF16Ptr("AIBrowserTray")

	// 윈도우 클래스 등록
	wc := WNDCLASSEXW{
		LpfnWndProc:   syscall.NewCallback(trayWndProc),
		HInstance:     syscall.Handle(hInst),
		LpszClassName: className,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// 히든 윈도우 생성
	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(APP_NAME))),
		0, 0, 0, 0, 0, 0, 0, hInst, 0,
	)
	trayHWnd = syscall.Handle(hwnd)

	// 아이콘 로드: exe 임베딩 리소스에서 (IDI_APPICON=1)
	hIcon, _, _ := pLoadIconW.Call(hInst, uintptr(IDI_APPICON))
	if hIcon == 0 {
		// 폴백: 시스템 기본 아이콘
		hIcon, _, _ = pLoadIconW.Call(0, uintptr(IDI_APPLICATION))
	}

	// 트레이 아이콘 추가
	trayNID = NOTIFYICONDATAW{
		HWnd:             trayHWnd,
		UID:              1,
		UFlags:           NIF_ICON | NIF_TIP | NIF_MESSAGE,
		UCallbackMessage: WM_TRAYICON,
		HIcon:            syscall.Handle(hIcon),
	}
	trayNID.CbSize = uint32(unsafe.Sizeof(trayNID))
	tip := syscall.StringToUTF16(APP_NAME)
	copy(trayNID.SzTip[:], tip)
	pShell_NotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&trayNID)))

	// 메시지 루프
	var msg MSG
	for {
		ret, _, _ := pGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)), 0, 0, 0,
		)
		if ret == 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func trayWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAYICON:
		switch lParam {
		case WM_LBUTTONDBLCLK:
			showMainWindow()
		case WM_RBUTTONUP:
			showTrayMenu()
		}
		return 0
	case WM_COMMAND:
		menuID := int(wParam & 0xFFFF)
		handleMenuClick(menuID)
		return 0
	case WM_DESTROY:
		// 트레이 아이콘 제거
		pShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&trayNID)))
		pPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func showMainWindow() {
	if mainHWND != 0 {
		pShowWindow.Call(mainHWND, SW_SHOW)
		pShowWindow.Call(mainHWND, SW_RESTORE)
		pSetForegroundWindow.Call(mainHWND)
	}
}

func hideMainWindow() {
	if mainHWND != 0 {
		saveWindowState()
		saveState()
		pShowWindow.Call(mainHWND, SW_HIDE)
	}
}

func showTrayMenu() {
	hMenu, _, _ := pCreatePopupMenu.Call()

	// 메뉴 항목 추가
	pAppendMenuW.Call(hMenu, MF_STRING, MENU_SHOW,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("열기"))))
	pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)

	// AI 사이트별 메뉴
	for i, site := range allTabs {
		label := site.Name
		pAppendMenuW.Call(hMenu, MF_STRING, uintptr(MENU_SHOW+1+i),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(label))))
	}

	pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
	pAppendMenuW.Call(hMenu, MF_STRING, MENU_ABOUT,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("프로그램 정보"))))
	pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
	pAppendMenuW.Call(hMenu, MF_STRING, MENU_QUIT,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("종료"))))

	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(uintptr(trayHWnd))
	pTrackPopupMenu.Call(hMenu, TPM_BOTTOMALIGN|TPM_LEFTALIGN,
		uintptr(pt.X), uintptr(pt.Y), 0, uintptr(trayHWnd), 0)
	pDestroyMenu.Call(hMenu)
}

func handleMenuClick(menuID int) {
	if menuID == MENU_QUIT {
		if onQuit != nil {
			onQuit()
		}
		return
	}
	if menuID == MENU_ABOUT {
		showAboutDialog()
		return
	}
	if menuID == MENU_SHOW {
		showMainWindow()
		return
	}
	// AI 사이트 메뉴 (MENU_SHOW+1 ~ ...)
	idx := menuID - MENU_SHOW - 1
	if idx >= 0 && idx < len(allTabs) {
		showMainWindow()
		if navigateCallback != nil {
			navigateCallback(idx)
		}
	}
}

// 프로그램 정보 대화상자
func showAboutDialog() {
	msg := fmt.Sprintf("%s v%s\n\n개발자: %s", APP_NAME, APP_VERSION, APP_DEVELOPER)
	title := APP_NAME + " 정보"
	pMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(msg))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))),
		MB_OK|MB_ICONINFORMATION,
	)
}

// 트레이 정리
func removeTray() {
	pShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&trayNID)))
}

// 탭 전환 콜백 (main.go에서 설정, 탭 인덱스 기반)
var navigateCallback func(idx int)

// AppUserModelID 설정 (작업표시줄 아이콘 고정)
func setAppUserModelID(appID string) {
	proc := shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	proc.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(appID))))
}
