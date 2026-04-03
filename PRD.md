# AI Browser v2 - PRD (Product Requirements Document)

## 개요
여러 AI 서비스(Claude, Gemini, ChatGPT, Grok 등)를 하나의 브라우저 탭 UI로 모아보는 데스크톱 앱

## 기술 스택
- **프레임워크**: Tauri v2 (Rust + WebView2)
- **프론트엔드**: Vanilla HTML/CSS/JS
- **빌드**: NSIS 인스톨러 (Windows)
- **설정 저장**: `%APPDATA%/AIBrowser/tabs.json`

## 핵심 기능

### 1. 탭 브라우징
- 상단 탭바에서 AI 서비스 간 원클릭 전환
- 각 탭은 독립 WebView (세션/쿠키 분리)
- 탭 클릭 시 해당 서비스 홈으로 이동
- 뒤로/앞으로/새로고침 네비게이션

### 2. 탭 관리
- 프리셋: Claude, Gemini, ChatGPT, Grok, Perplexity, Copilot 등 12개
- 사용자 직접 URL 추가
- 탭 순서 변경 (▲▼)
- 탭 삭제 (최소 1개 유지)

### 3. 시스템 트레이
- 닫기 버튼 → 트레이로 최소화
- 트레이 더블클릭 → 창 복원
- 우클릭 메뉴: 열기/프로그램 정보/종료

### 4. 자동 업데이트 알림
- GitHub Releases API로 최신 버전 확인
- 새 버전 존재 시 다운로드 페이지 안내

### 5. 같은 탭 열기
- `target="_blank"` 링크를 현재 탭에서 열기
- `window.open()` 오버라이드

## 알려진 이슈 (Known Issues)

| # | 이슈 | 상태 | 비고 |
|---|------|------|------|
| 1 | 네이버 메일탭에서 메일 바로 읽기 안열림 | 🔴 미해결 | iframe/CSP 제한 추정 |

## 버전 히스토리

| 버전 | 날짜 | 내용 |
|------|------|------|
| v2.0.0 | 2026-04 | 초기 릴리스 (Tauri v2) |
| v2.0.1 | 2026-04-04 | 명세서 정리, 알려진 이슈 문서화 |

## 제작
- **개발자**: 혜통
- **식별자**: com.hyetong.aibrowser
- **GitHub**: ReachToWisdom/AI-Browser
