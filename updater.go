package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// GitHub 릴리스 정보
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// GitHub 리포지토리 설정
const (
	GITHUB_OWNER = "ReachToWisdom"
	GITHUB_REPO  = "AI-Browser"
)

// 업데이트 확인 (백그라운드)
func startUpdateChecker() {
	// 시작 후 10초 대기 (앱 초기화 완료 후)
	time.Sleep(10 * time.Second)
	checkForUpdate()
}

// GitHub Releases API로 최신 버전 확인
func checkForUpdate() {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		GITHUB_OWNER, GITHUB_REPO)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return // 네트워크 오류 시 무시
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return
	}

	// 버전 비교 (v 접두사 제거)
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == "" || latestVersion == APP_VERSION {
		return
	}

	// 최신 버전이 현재보다 높은지 비교
	if !isNewerVersion(latestVersion, APP_VERSION) {
		return
	}

	// 업데이트 알림 표시
	showUpdateDialog(latestVersion, release.HTMLURL)
}

// 단순 버전 비교 (x.y.z 형식)
func isNewerVersion(latest, current string) bool {
	lParts := strings.Split(latest, ".")
	cParts := strings.Split(current, ".")

	maxLen := len(lParts)
	if len(cParts) > maxLen {
		maxLen = len(cParts)
	}

	for i := 0; i < maxLen; i++ {
		var l, c int
		if i < len(lParts) {
			fmt.Sscanf(lParts[i], "%d", &l)
		}
		if i < len(cParts) {
			fmt.Sscanf(cParts[i], "%d", &c)
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

// 업데이트 알림 대화상자
func showUpdateDialog(newVersion, downloadURL string) {
	msg := fmt.Sprintf("새 버전이 있습니다!\n\n현재: v%s\n최신: v%s\n\n다운로드 페이지를 열까요?",
		APP_VERSION, newVersion)
	title := APP_NAME + " 업데이트"

	const MB_YESNO = 0x00000004
	const MB_SYSTEMMODAL = 0x00001000 // 최상단 표시
	const IDYES = 6

	ret, _, _ := pMessageBoxW.Call(
		mainHWND,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(msg))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))),
		MB_YESNO|MB_ICONINFORMATION|MB_SYSTEMMODAL,
	)

	if ret == IDYES {
		// 기본 브라우저로 다운로드 페이지 열기
		openURLInBrowser(downloadURL)
	}
}

// 기본 브라우저로 URL 열기
func openURLInBrowser(url string) {
	shell32.NewProc("ShellExecuteW").Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("open"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(url))),
		0, 0, 1,
	)
}
