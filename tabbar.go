package main

import (
	"syscall"
	"unsafe"
)

var (
	gdi32             = syscall.NewLazyDLL("gdi32.dll")
	pBeginPaint       = user32.NewProc("BeginPaint")
	pEndPaint         = user32.NewProc("EndPaint")
	pFillRect         = user32.NewProc("FillRect")
	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject     = gdi32.NewProc("DeleteObject")
	pSetBkMode        = gdi32.NewProc("SetBkMode")
	pSetTextColor     = gdi32.NewProc("SetTextColor")
	pTextOutW         = gdi32.NewProc("TextOutW")
	pSelectObject     = gdi32.NewProc("SelectObject")
	pCreateFontW      = gdi32.NewProc("CreateFontW")
	pEllipse          = gdi32.NewProc("Ellipse")
	pSetWindowPos     = user32.NewProc("SetWindowPos")
	pClientToScreen   = user32.NewProc("ClientToScreen")
)

const (
	WM_PAINT        = 0x000F
	WM_LBUTTONDOWN  = 0x0201
	WS_CHILD        = 0x40000000
	WS_VISIBLE      = 0x10000000
	WS_CLIPCHILDREN = 0x02000000
	TRANSPARENT_BG  = 1
	HWND_TOP        = 0
	SWP_SHOWWINDOW  = 0x0040
	TAB_MIN_W       = 100 // 탭 최소 너비
	TAB_MAX_W       = 220 // 탭 최대 너비
	REFRESH_W       = 40  // 새로고침 버튼 너비 (좌측)
	GEAR_W          = 40  // 설정 버튼 너비 (우측)
	MENU_TAB_DELETE = 2000
)

type PAINTSTRUCT struct {
	HDC         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

var tabBarHWND syscall.Handle

func createTabBar(parentHWND uintptr) {
	hInst, _, _ := pGetModuleHandleW.Call(0)
	className := syscall.StringToUTF16Ptr("AIBrowserTabBar")
	bgBrush, _, _ := pCreateSolidBrush.Call(0x002E1E1E)
	wc := WNDCLASSEXW{
		LpfnWndProc:   syscall.NewCallback(tabBarWndProc),
		HInstance:     syscall.Handle(hInst),
		LpszClassName: className,
		HbrBackground: syscall.Handle(bgBrush),
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	hwnd, _, _ := pCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), 0,
		WS_CHILD|WS_VISIBLE|WS_CLIPCHILDREN,
		0, 0, WINDOW_W, uintptr(TABBAR_H),
		parentHWND, 0, hInst, 0,
	)
	tabBarHWND = syscall.Handle(hwnd)
	pSetWindowPos.Call(hwnd, HWND_TOP, 0, 0, 0, 0, SWP_SHOWWINDOW|0x0001|0x0002)
}

func tabBarWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_PAINT:
		paintTabBar(hwnd)
		return 0
	case WM_LBUTTONDOWN:
		handleTabClick(int(lParam & 0xFFFF))
		return 0
	case WM_RBUTTONUP:
		// 우클릭 → 컨텍스트 메뉴
		handleTabRightClick(hwnd, int(lParam&0xFFFF), int((lParam>>16)&0xFFFF))
		return 0
	case WM_COMMAND:
		menuID := int(wParam & 0xFFFF)
		if menuID == MENU_TAB_DELETE {
			handleTabDelete()
		}
		return 0
	}
	ret, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

// 우클릭한 탭 인덱스 저장
var rightClickedTab int = -1

func handleTabRightClick(hwnd syscall.Handle, x, y int) {
	var barRect RECT
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&barRect)))
	barWidth := int(barRect.Right)

	tabMutex.Lock()
	tabCount := len(allTabs)
	tabMutex.Unlock()

	tabW := calcTabWidth(barWidth, tabCount)
	idx := (x - REFRESH_W) / tabW
	if idx < 0 || idx >= tabCount {
		return
	}
	rightClickedTab = idx

	// 컨텍스트 메뉴 생성
	hMenu, _, _ := pCreatePopupMenu.Call()

	tabMutex.Lock()
	tabName := allTabs[idx].Name
	canDelete := len(allTabs) > 1
	tabMutex.Unlock()

	label := "\"" + tabName + "\" 탭 삭제"
	if canDelete {
		pAppendMenuW.Call(hMenu, MF_STRING, MENU_TAB_DELETE,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(label))))
	} else {
		// 마지막 1개는 삭제 불가
		pAppendMenuW.Call(hMenu, MF_STRING|0x0002, MENU_TAB_DELETE,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(label+" (최소 1개 필요)"))))
	}

	// 화면 좌표로 변환
	var pt POINT
	pt.X, pt.Y = int32(x), int32(y)
	pClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pt)))

	pSetForegroundWindow.Call(uintptr(hwnd))
	pTrackPopupMenu.Call(hMenu, TPM_LEFTALIGN|0x0020,
		uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hwnd), 0)
	pDestroyMenu.Call(hMenu)
}

func handleTabDelete() {
	if rightClickedTab < 0 {
		return
	}
	idx := rightClickedTab
	rightClickedTab = -1

	if removeTabAt(idx) {
		refreshTabBar()
		if navigateCallback != nil {
			navigateCallback(activeTab)
		}
	}
}

func paintTabBar(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	// 탭 텍스트 폰트 (15px)
	font, _, _ := pCreateFontW.Call(
		uintptr(uint32(0xFFFFFFEB)), 0, 0, 0, 500, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Segoe UI"))),
	)
	oldFont, _, _ := pSelectObject.Call(hdc, font)
	pSetBkMode.Call(hdc, TRANSPARENT_BG)

	var barRect RECT
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&barRect)))
	barWidth := int(barRect.Right)

	tabMutex.Lock()
	tabs := make([]TabItem, len(allTabs))
	copy(tabs, allTabs)
	curActive := activeTab
	tabMutex.Unlock()

	// 아이콘용 폰트 (20px)
	iconFont, _, _ := pCreateFontW.Call(
		uintptr(uint32(0xFFFFFFE6)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Segoe UI Symbol"))),
	)

	// 🔄 새로고침 버튼 (좌측 끝)
	pSetTextColor.Call(hdc, 0x00BBBBBB)
	oldIcon, _, _ := pSelectObject.Call(hdc, iconFont)
	refresh := syscall.StringToUTF16("\u21BB") // ↻
	pTextOutW.Call(hdc, uintptr(10), uintptr((TABBAR_H-22)/2),
		uintptr(unsafe.Pointer(&refresh[0])), 1)
	pSelectObject.Call(hdc, oldIcon)

	// 동적 탭 너비 계산
	tabW := calcTabWidth(barWidth, len(tabs))

	// 탭 목록
	for i, tab := range tabs {
		x := REFRESH_W + i*tabW

		if i == curActive {
			brush, _, _ := pCreateSolidBrush.Call(0x005A4745)
			r := RECT{int32(x), 0, int32(x + tabW), int32(TABBAR_H)}
			pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
			pDeleteObject.Call(brush)
			pSetTextColor.Call(hdc, 0x00A1E3A6)
		} else {
			pSetTextColor.Call(hdc, 0x00C8ADA6)
		}

		// 색상 원 (10px)
		dotBrush, _, _ := pCreateSolidBrush.Call(uintptr(tab.ColorBGR))
		old, _, _ := pSelectObject.Call(hdc, dotBrush)
		dotX, dotY := x+10, (TABBAR_H-10)/2
		pEllipse.Call(hdc, uintptr(dotX), uintptr(dotY), uintptr(dotX+10), uintptr(dotY+10))
		pSelectObject.Call(hdc, old)
		pDeleteObject.Call(dotBrush)

		// 탭 이름
		nameStr := tab.Name
		maxTextW := tabW - 36
		if maxTextW < 20 {
			maxTextW = 20
		}
		maxChars := maxTextW / 9
		if len([]rune(nameStr)) > maxChars && maxChars > 3 {
			runes := []rune(nameStr)
			nameStr = string(runes[:maxChars-1]) + "…"
		}
		name := syscall.StringToUTF16(nameStr)
		pTextOutW.Call(hdc, uintptr(dotX+16), uintptr((TABBAR_H-16)/2),
			uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)-1))
	}

	// ⚙ 설정 버튼 (우측 끝)
	pSetTextColor.Call(hdc, 0x00BBBBBB)
	oldIcon2, _, _ := pSelectObject.Call(hdc, iconFont)
	gear := syscall.StringToUTF16("\u2699") // ⚙
	gearX := barWidth - GEAR_W + 10
	pTextOutW.Call(hdc, uintptr(gearX), uintptr((TABBAR_H-22)/2),
		uintptr(unsafe.Pointer(&gear[0])), 1)
	pSelectObject.Call(hdc, oldIcon2)

	pDeleteObject.Call(iconFont)
	pSelectObject.Call(hdc, oldFont)
	pDeleteObject.Call(font)
	pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
}

func handleTabClick(x int) {
	// 탭바 너비 계산
	var barRect RECT
	pGetClientRect.Call(uintptr(tabBarHWND), uintptr(unsafe.Pointer(&barRect)))
	barWidth := int(barRect.Right)

	// 새로고침 버튼 (좌측 끝, 0 ~ REFRESH_W)
	if x < REFRESH_W {
		reloadCurrentTab()
		return
	}

	// 설정 버튼 (우측 끝, barWidth-GEAR_W ~ barWidth)
	if x >= barWidth-GEAR_W {
		openSettings()
		return
	}

	tabMutex.Lock()
	tabCount := len(allTabs)
	tabMutex.Unlock()

	// 동적 탭 너비로 인덱스 계산
	tabW := calcTabWidth(barWidth, tabCount)
	idx := (x - REFRESH_W) / tabW
	if idx < 0 || idx >= tabCount {
		return
	}

	if navigateCallback != nil {
		navigateCallback(idx)
	}
}

// 새로고침: 현재 탭 페이지 새로고침
func reloadCurrentTab() {
	if activeTab < 0 || activeTab >= len(tabChromiums) {
		return
	}
	// 설정 페이지에서는 탭으로 복귀
	if onSettingsPage {
		closeSettingsView()
		return
	}
	if c := tabChromiums[activeTab]; c != nil {
		// ICoreWebView2.Reload() vtable 호출
		wv := getWebView(c)
		if wv != 0 {
			const vtblReload = 31
			syscall.SyscallN(comFn(wv, vtblReload), wv)
		}
	}
}

func refreshTabBar() {
	pInvalidateRect.Call(uintptr(tabBarHWND), 0, 1)
}

// 탭바 너비와 탭 수에 따라 동적 탭 너비 계산
func calcTabWidth(barWidth, tabCount int) int {
	if tabCount <= 0 {
		return TAB_MAX_W
	}
	available := barWidth - REFRESH_W - GEAR_W
	w := available / tabCount
	if w < TAB_MIN_W {
		return TAB_MIN_W
	}
	if w > TAB_MAX_W {
		return TAB_MAX_W
	}
	return w
}

func myMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
