package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// URL 정규화 (프로토콜 없으면 https:// 추가)
func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return u
	}
	if !strings.Contains(u, "://") {
		return "https://" + u
	}
	return u
}

// 탭 설정
type TabItem struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	ColorBGR   uint32 `json:"color"`                // 탭 색상 (BGR)
	CurrentURL string `json:"currentUrl,omitempty"` // 마지막 방문 URL (앱 재시작 시 복원)
}

// 설정 페이지 표시 중 여부
var onSettingsPage bool

// 기본 탭 (최초 실행 시 사용)
var defaultTabs = []TabItem{
	{Name: "Claude", URL: "https://claude.ai", ColorBGR: 0x0032A0FF},
	{Name: "Gemini", URL: "https://gemini.google.com", ColorBGR: 0x00F48542},
	{Name: "ChatGPT", URL: "https://chatgpt.com", ColorBGR: 0x007FA310},
	{Name: "Grok", URL: "https://grok.com", ColorBGR: 0x00FF648C},
}

// 창 상태 (앱 재시작 시 복원)
type AppState struct {
	WindowX   int `json:"windowX"`
	WindowY   int `json:"windowY"`
	WindowW   int `json:"windowW"`
	WindowH   int `json:"windowH"`
	ActiveTab int `json:"activeTab"`
}

var (
	allTabs    []TabItem
	tabMutex   sync.Mutex
	activeTab  int
	configPath string
	statePath  string
	appState   AppState
)

// 앱 데이터 디렉토리 (%APPDATA%/AIBrowser, 캐싱)
var appDir string

func getAppDir() string {
	if appDir != "" {
		return appDir
	}
	// %APPDATA%/AIBrowser (exe 위치 무관, 고정 경로)
	appData := os.Getenv("APPDATA")
	if appData == "" {
		// 폴백: exe 위치 기준
		exe, _ := os.Executable()
		appDir = filepath.Dir(exe)
		return appDir
	}
	appDir = filepath.Join(appData, "AIBrowser")
	os.MkdirAll(appDir, 0755)
	return appDir
}

// 설정 파일 경로
func initConfigPath() {
	dir := getAppDir()
	configPath = filepath.Join(dir, "tabs.json")
	statePath = filepath.Join(dir, "state.json")
}

// 탭 로드 (없으면 기본값 생성)
func loadTabs() {
	initConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		allTabs = make([]TabItem, len(defaultTabs))
		copy(allTabs, defaultTabs)
		saveTabs()
		return
	}
	if err := json.Unmarshal(data, &allTabs); err != nil || len(allTabs) == 0 {
		allTabs = make([]TabItem, len(defaultTabs))
		copy(allTabs, defaultTabs)
		saveTabs()
	}
}

// 탭 저장
func saveTabs() {
	data, _ := json.MarshalIndent(allTabs, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

// 창 상태 로드
func loadState() {
	appState = AppState{
		WindowW: WINDOW_W,
		WindowH: WINDOW_H,
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &appState)
	// 최소 크기 보장
	if appState.WindowW < 400 {
		appState.WindowW = WINDOW_W
	}
	if appState.WindowH < 300 {
		appState.WindowH = WINDOW_H
	}
	// 화면 밖 좌표 보정 (RustDesk/모니터 변경 시 창이 안 보이는 문제 방지)
	screenW, screenH := getScreenSize()
	if appState.WindowX < -100 || appState.WindowX > screenW-100 {
		appState.WindowX = 100
	}
	if appState.WindowY < -50 || appState.WindowY > screenH-100 {
		appState.WindowY = 100
	}
}

// 창 상태 저장
func saveState() {
	appState.ActiveTab = activeTab
	data, _ := json.MarshalIndent(appState, "", "  ")
	os.WriteFile(statePath, data, 0644)
}

func addTab(name, url string) int {
	return addTabWithColor(name, url, 0x00888888)
}

func addTabWithColor(name, url string, color uint32) int {
	tabMutex.Lock()
	defer tabMutex.Unlock()
	if color == 0 {
		color = 0x00888888
	}
	// URL에 프로토콜이 없으면 https:// 자동 추가
	url = normalizeURL(url)
	allTabs = append(allTabs, TabItem{
		Name: name, URL: url, ColorBGR: color,
	})
	onTabAdded()
	saveTabs()
	return len(allTabs) - 1
}

// 탭 순서 변경 (fromIdx → toIdx로 이동)
func reorderTab(fromIdx, toIdx int) {
	tabMutex.Lock()
	defer tabMutex.Unlock()
	if fromIdx < 0 || fromIdx >= len(allTabs) || toIdx < 0 || toIdx >= len(allTabs) || fromIdx == toIdx {
		return
	}
	tab := allTabs[fromIdx]
	// 기존 위치에서 제거
	allTabs = append(allTabs[:fromIdx], allTabs[fromIdx+1:]...)
	// 새 위치에 삽입
	allTabs = append(allTabs[:toIdx], append([]TabItem{tab}, allTabs[toIdx:]...)...)
	// Chromium 슬라이스도 동기화
	onTabReordered(fromIdx, toIdx)
	// 활성 탭 인덱스 보정
	if activeTab == fromIdx {
		activeTab = toIdx
	} else if fromIdx < activeTab && toIdx >= activeTab {
		activeTab--
	} else if fromIdx > activeTab && toIdx <= activeTab {
		activeTab++
	}
	saveTabs()
}

func removeTabAt(idx int) bool {
	tabMutex.Lock()
	defer tabMutex.Unlock()
	if idx < 0 || idx >= len(allTabs) || len(allTabs) <= 1 {
		return false // 최소 1개는 유지
	}
	allTabs = append(allTabs[:idx], allTabs[idx+1:]...)
	onTabRemoved(idx)
	if activeTab >= len(allTabs) {
		activeTab = len(allTabs) - 1
	}
	saveTabs()
	return true
}

// 프리셋 목록 (설정 페이지에서 원클릭 추가용)
var presetTabs = []TabItem{
	{Name: "Claude", URL: "https://claude.ai", ColorBGR: 0x0032A0FF},
	{Name: "Gemini", URL: "https://gemini.google.com", ColorBGR: 0x00F48542},
	{Name: "ChatGPT", URL: "https://chatgpt.com", ColorBGR: 0x007FA310},
	{Name: "Grok", URL: "https://grok.com", ColorBGR: 0x00FF648C},
	{Name: "Perplexity", URL: "https://perplexity.ai", ColorBGR: 0x00E8A020},
	{Name: "Copilot", URL: "https://copilot.microsoft.com", ColorBGR: 0x00E8B644},
	{Name: "NotebookLM", URL: "https://notebooklm.google.com", ColorBGR: 0x0042A5F5},
	{Name: "AI Studio", URL: "https://aistudio.google.com", ColorBGR: 0x004285F4},
	{Name: "Poe", URL: "https://poe.com", ColorBGR: 0x00CC9955},
	{Name: "HuggingChat", URL: "https://huggingface.co/chat", ColorBGR: 0x0000D4FF},
	{Name: "DeepSeek", URL: "https://chat.deepseek.com", ColorBGR: 0x00FF8C4B},
	{Name: "Mistral", URL: "https://chat.mistral.ai", ColorBGR: 0x002BACFF},
}

// 화면 크기 조회 (GetSystemMetrics)
func getScreenSize() (int, int) {
	pGetSystemMetrics := user32.NewProc("GetSystemMetrics")
	w, _, _ := pGetSystemMetrics.Call(0) // SM_CXSCREEN
	h, _, _ := pGetSystemMetrics.Call(1) // SM_CYSCREEN
	if w == 0 || h == 0 {
		return 1920, 1080 // 폴백
	}
	return int(w), int(h)
}

// 하위 호환 (tray.go 등에서 AI_SITES 참조)
type AISite = TabItem

var AI_SITES = defaultTabs

const (
	APP_NAME      = "AI Browser"
	APP_VERSION   = "1.2.0"
	APP_DEVELOPER = "혜통"
	WINDOW_W      = 1400
	WINDOW_H      = 900
	TABBAR_H      = 48
	WM_TRAYICON   = 0x8001
	IDI_APPICON   = 1
)
