# AI Browser - 제품 요구사항 명세서

## 개요
AI 서비스 전용 브라우저. Claude, Gemini, ChatGPT, Grok 4개 사이트를 고정 탭으로 제공.
일반 브라우저 창과 혼동 방지가 핵심 목적.

## 문제
- 일반 브라우저에서 AI 탭을 열면 다른 창과 혼동
- 실수로 닫았다 살렸다 반복
- AI 대화 세션이 끊기는 불편함

## 해결
- AI 전용 독립 실행파일 (.exe)
- 4개 AI 사이트 고정 탭
- X 버튼 = 트레이로 숨기기 (종료 아님)
- 트레이 메뉴에서만 실제 종료 가능

## 기술 스택
- **언어**: Go 1.24+
- **UI 엔진**: WebView2 (Edge 기반, Windows 11 기본 탑재)
- **라이브러리**: jchv/go-webview2 (순수 Go, CGO 불필요)
- **시스템 트레이**: Win32 API 직접 호출 (golang.org/x/sys/windows)
- **빌드**: `go build` → 단독 .exe

## 핵심 기능

### 1. 고정 탭 (4개)
| 탭 | URL | 아이콘 |
|----|-----|--------|
| Claude | https://claude.ai | 🟠 |
| Gemini | https://gemini.google.com | 🔵 |
| ChatGPT | https://chatgpt.com | 🟢 |
| Grok | https://grok.com | ⚡ |

### 2. 탭 전환
- 상단 고정 탭바 (JS 인젝션, 모든 페이지에 오버레이)
- 탭 클릭 → WebView2 Navigate()로 해당 사이트 이동
- 쿠키/세션 유지 (WebView2 프로필 자동 관리)

### 3. 트레이 동작
- X 버튼 → 트레이로 최소화 (WM_CLOSE 가로채기)
- 트레이 더블클릭 → 창 복원
- 트레이 우클릭 메뉴: 열기 / 각 AI 탭 / 종료

### 4. 단일 인스턴스
- 뮤텍스로 중복 실행 방지
- 이미 실행 중이면 기존 창 활성화

## 제약사항
- Windows 전용 (WebView2 + Win32 API)
- iframe 차단으로 동시 로딩 불가 → 탭 전환 시 페이지 리로드
- AI 사이트 로그인 상태는 WebView2 쿠키로 유지

## 폴더 구조
```
AI_Browser/
├── main.go       // 진입점
├── config.go     // AI 사이트 설정 (SSOT)
├── ui.go         // 탭바 HTML/CSS/JS
├── tray.go       // 시스템 트레이 (Win32)
├── go.mod
├── CLAUDE.md
├── PRD.md
└── Tasks.md
```
