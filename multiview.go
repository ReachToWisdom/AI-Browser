package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
)

// 탭별 독립 WebView2 인스턴스 (모든 탭이 자체 WebView2 보유)
var tabChromiums []*edge.Chromium

// 탭 전환 중 재진입 방지
var tabSwitching bool

// 탭 뷰 초기화
func initTabViews() {
	tabChromiums = make([]*edge.Chromium, len(allTabs))
}

// 탭용 WebView2 생성 (지연 생성: 첫 전환 시)
func ensureTabChromium(idx int) *edge.Chromium {
	if idx < 0 || idx >= len(allTabs) {
		return nil
	}
	for len(tabChromiums) <= idx {
		tabChromiums = append(tabChromiums, nil)
	}
	if tabChromiums[idx] != nil {
		return tabChromiums[idx]
	}

	c := edge.NewChromium()

	// 공유 세션: 모든 탭이 동일 세션 (속도 최우선)
	c.DataPath = filepath.Join(getAppDir(), "data", "shared")

	c.SetPermission(edge.CoreWebView2PermissionKindClipboardRead, edge.CoreWebView2PermissionStateAllow)

	// 메시지 콜백 (URL 추적 + 외부 링크)
	tabIdx := idx
	c.MessageCallback = func(msg string) {
		handleTabMessage(tabIdx, msg)
	}

	if !c.Embed(mainHWND) {
		return nil
	}

	// 디버그 도구 비활성화
	if settings, err := c.GetSettings(); err == nil {
		settings.PutAreDefaultContextMenusEnabled(false)
		settings.PutAreDevToolsEnabled(false)
	}

	c.Hide()
	c.Init(generateTabJS())

	// 저장된 URL 또는 기본 URL로 이동
	tabMutex.Lock()
	targetURL := allTabs[idx].CurrentURL
	if targetURL == "" {
		targetURL = allTabs[idx].URL
	}
	tabMutex.Unlock()
	c.Navigate(targetURL)

	setChromiumBounds(c)
	tabChromiums[idx] = c

	// 이벤트 핸들러 등록 (팝업 → 외부 브라우저, 크래시 → 자동 복구)
	hookTabEvents(c)

	return c
}

// 탭 전환 (핵심 함수: 이전 탭 숨김 → 새 탭 표시)
func switchToTab(newIdx int) {
	if tabSwitching {
		return
	}
	tabSwitching = true
	defer func() { tabSwitching = false }()

	tabMutex.Lock()
	if newIdx < 0 || newIdx >= len(allTabs) {
		tabMutex.Unlock()
		return
	}
	tabMutex.Unlock()

	// 같은 탭 클릭 → Chromium이 이미 있으면 홈 URL로 복귀, 없으면 새로 생성
	if newIdx == activeTab && !onSettingsPage {
		if newIdx < len(tabChromiums) && tabChromiums[newIdx] != nil {
			tabMutex.Lock()
			homeURL := allTabs[newIdx].URL
			tabMutex.Unlock()
			tabChromiums[newIdx].Navigate(homeURL)
			return
		}
		// Chromium 미생성 → 아래 로직으로 진행하여 생성
	}

	// 새 탭 먼저 준비 → Show → 이전 탭 Hide (깜박임 방지)
	prevTab := activeTab
	prevOnSettings := onSettingsPage

	onSettingsPage = false
	activeTab = newIdx

	c := ensureTabChromium(newIdx)
	if c != nil {
		setChromiumBounds(c)
		c.Show()
		c.Focus()
	}

	// 이전 탭 숨김 (새 탭이 보인 후)
	if prevOnSettings {
		putMainWebViewVisible(false)
	} else {
		hideTabChromium(prevTab)
	}

	refreshTabBar()
}

func hideTabChromium(idx int) {
	if idx >= 0 && idx < len(tabChromiums) && tabChromiums[idx] != nil {
		tabChromiums[idx].Hide()
	}
}

// Chromium bounds 설정 (탭바 아래 콘텐츠 영역)
func setChromiumBounds(c *edge.Chromium) {
	ctrl := c.GetController()
	if ctrl == nil {
		return
	}
	var rect RECT
	pGetClientRect.Call(mainHWND, uintptr(unsafe.Pointer(&rect)))
	bounds := RECT{0, int32(TABBAR_H), rect.Right, rect.Bottom}

	// ICoreWebView2Controller.PutBounds (vtable offset 6)
	ctrlPtr := uintptr(unsafe.Pointer(ctrl))
	vtblPtr := *(*uintptr)(unsafe.Pointer(ctrlPtr))
	fn := *(*uintptr)(unsafe.Pointer(vtblPtr + 6*unsafe.Sizeof(uintptr(0))))
	syscall.SyscallN(fn, ctrlPtr, uintptr(unsafe.Pointer(&bounds)))
}

// 모든 탭 리사이즈 (창 크기 변경 시)
func resizeSecondaryTabs() {
	for _, c := range tabChromiums {
		if c != nil {
			setChromiumBounds(c)
		}
	}
}

// 설정 페이지 열기 (메인 웹뷰 사용)
func openSettingsView() {
	onSettingsPage = true
	hideTabChromium(activeTab)
	putMainWebViewVisible(true)
	resizeMainWebView()
}

// 설정에서 복귀
func closeSettingsView() {
	onSettingsPage = false
	putMainWebViewVisible(false)
	if c := ensureTabChromium(activeTab); c != nil {
		setChromiumBounds(c)
		c.Show()
		c.Focus()
	}
}

// 보조 탭 메시지 처리 (postMessage 기반)
func handleTabMessage(tabIdx int, msg string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(msg), &data); err != nil {
		return
	}
	msgType, _ := data["type"].(string)
	switch msgType {
	case "urlChanged":
		url, _ := data["url"].(string)
		if url == "" || strings.HasPrefix(url, "data:") || url == "about:blank" {
			return
		}
		tabMutex.Lock()
		if tabIdx >= 0 && tabIdx < len(allTabs) {
			allTabs[tabIdx].CurrentURL = url
			saveTabs()
		}
		tabMutex.Unlock()
	}
}

// 보조 탭용 JS (URL 추적만)
func generateTabJS() string {
	return `(function(){
function r(){try{var u=location.href;
if(u&&u.indexOf('data:')!==0&&u!=='about:blank')
window.chrome.webview.postMessage(JSON.stringify({type:'urlChanged',url:u}))}catch(e){}}
addEventListener('load',r);
var p=history.pushState;history.pushState=function(){p.apply(this,arguments);r()};
var s=history.replaceState;history.replaceState=function(){s.apply(this,arguments);r()};
addEventListener('popstate',r);r();
})();`
}

// 탭 추가/삭제/재정렬 시 tabChromiums 동기화
func onTabAdded() {
	tabChromiums = append(tabChromiums, nil)
}

func onTabRemoved(idx int) {
	if idx >= 0 && idx < len(tabChromiums) {
		tabChromiums = append(tabChromiums[:idx], tabChromiums[idx+1:]...)
	}
}

func onTabReordered(fromIdx, toIdx int) {
	n := len(tabChromiums)
	if fromIdx < 0 || fromIdx >= n || toIdx < 0 || toIdx >= n || fromIdx == toIdx {
		return
	}
	c := tabChromiums[fromIdx]
	tabChromiums = append(tabChromiums[:fromIdx], tabChromiums[fromIdx+1:]...)
	tabChromiums = append(tabChromiums[:toIdx], append([]*edge.Chromium{c}, tabChromiums[toIdx:]...)...)
}
