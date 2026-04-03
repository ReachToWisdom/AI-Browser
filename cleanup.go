package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 캐시 정리 설정
const (
	// 사용자 추가 탭 데이터 보관 기간 (15일)
	CUSTOM_TAB_MAX_AGE = 15 * 24 * time.Hour
)

// 삭제 대상 캐시 폴더 (로그인/쿠키 무관, 안전하게 삭제 가능)
var cacheDeleteTargets = []string{
	// EBWebView 루트
	"ShaderCache",
	"GraphiteDawnCache",
	"GrShaderCache",
	"Crashpad",
	"BrowserMetrics-spare.pma",
	// EBWebView/Default 내부
	"Default/Cache",
	"Default/Code Cache",
	"Default/GPUCache",
	"Default/DawnWebGPUCache",
	"Default/DawnGraphiteCache",
}

// 프리셋 AI 사이트 도메인 목록 (이 도메인은 캐시만 삭제, 로그인 유지)
var aiDomains = map[string]bool{
	"claude.ai":              true,
	"gemini.google.com":      true,
	"chatgpt.com":            true,
	"grok.com":               true,
	"perplexity.ai":          true,
	"copilot.microsoft.com":  true,
	"notebooklm.google.com":  true,
	"aistudio.google.com":    true,
	"poe.com":                true,
	"huggingface.co":         true,
	"chat.deepseek.com":      true,
	"chat.mistral.ai":        true,
}

// 앱 시작 시 캐시 정리 실행
func runCleanup() {
	var totalCleaned int64

	// 메인 데이터 디렉토리 정리
	dataDir := filepath.Join(getAppDir(), "data")
	totalCleaned += cleanDataDir(dataDir)

	// exe 위치의 구 data 폴더 정리 (이전 버전 잔여물)
	exe, err := os.Executable()
	if err == nil {
		oldDataDir := filepath.Join(filepath.Dir(exe), "data")
		if oldDataDir != dataDir {
			if info, err := os.Stat(oldDataDir); err == nil && info.IsDir() {
				size := dirSize(oldDataDir)
				if err := os.RemoveAll(oldDataDir); err == nil {
					totalCleaned += size
					fmt.Printf("[정리] 구 버전 데이터 삭제: %s (%.1fMB)\n",
						oldDataDir, float64(size)/1024/1024)
				}
			}
		}
	}

	if totalCleaned > 0 {
		fmt.Printf("[정리] 총 %.1fMB 정리 완료\n", float64(totalCleaned)/1024/1024)
	}
}

// 데이터 디렉토리 내 탭별 정리
func cleanDataDir(dataDir string) int64 {
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return 0
	}

	activeFolders := getActiveTabFolders()

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return 0
	}

	var totalCleaned int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folderName := entry.Name()
		folderPath := filepath.Join(dataDir, folderName)

		if _, active := activeFolders[folderName]; active {
			// 활성 탭: AI인지 사용자 추가인지 구분
			if isAIDomain(folderName) {
				// AI 탭: 캐시만 삭제, 로그인 유지
				totalCleaned += cleanTabCache(folderPath)
			} else {
				// 사용자 추가 탭: 15일 초과 시 전체 삭제, 아니면 캐시만
				if isOlderThan(folderPath, CUSTOM_TAB_MAX_AGE) {
					size := dirSize(folderPath)
					if err := os.RemoveAll(folderPath); err == nil {
						totalCleaned += size
						fmt.Printf("[정리] 사용자 탭 데이터 삭제 (15일 초과): %s (%.1fMB)\n",
							folderName, float64(size)/1024/1024)
					}
				} else {
					totalCleaned += cleanTabCache(folderPath)
				}
			}
		} else {
			// 고아 폴더 (탭이 삭제되었지만 데이터 남음): 전체 삭제
			size := dirSize(folderPath)
			if err := os.RemoveAll(folderPath); err == nil {
				totalCleaned += size
				fmt.Printf("[정리] 고아 데이터 삭제: %s (%.1fMB)\n",
					folderName, float64(size)/1024/1024)
			}
		}
	}
	return totalCleaned
}

// 탭별 캐시 폴더만 삭제 (쿠키/로그인 유지)
func cleanTabCache(folderPath string) int64 {
	ebwebview := filepath.Join(folderPath, "EBWebView")
	if _, err := os.Stat(ebwebview); os.IsNotExist(err) {
		return 0
	}

	var cleaned int64
	for _, target := range cacheDeleteTargets {
		targetPath := filepath.Join(ebwebview, target)
		info, err := os.Stat(targetPath)
		if os.IsNotExist(err) {
			continue
		}
		if info.IsDir() {
			size := dirSize(targetPath)
			if err := os.RemoveAll(targetPath); err == nil {
				cleaned += size
			}
		} else {
			size := info.Size()
			if err := os.Remove(targetPath); err == nil {
				cleaned += size
			}
		}
	}
	return cleaned
}

// 현재 탭 목록에서 폴더명 추출
func getActiveTabFolders() map[string]bool {
	tabMutex.Lock()
	defer tabMutex.Unlock()

	folders := make(map[string]bool, len(allTabs))
	for _, tab := range allTabs {
		parsed, err := url.Parse(tab.URL)
		if err != nil {
			continue
		}
		folderName := parsed.Hostname()
		safePath := strings.ReplaceAll(strings.Trim(parsed.Path, "/"), "/", "_")
		if safePath != "" {
			folderName += "_" + safePath
		}
		folders[folderName] = true
	}
	return folders
}

// 도메인이 AI 프리셋인지 확인
func isAIDomain(folderName string) bool {
	// 폴더명에서 도메인 추출 (경로 부분 제거)
	domain := strings.Split(folderName, "_")[0]
	return aiDomains[domain]
}

// 폴더 최종 수정 시간이 기준보다 오래됐는지 확인
func isOlderThan(path string, maxAge time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > maxAge
}

// 폴더 전체 크기 계산
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
