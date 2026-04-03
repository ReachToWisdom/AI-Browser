# AI Browser - 작업 목록

## Phase 1: 프로젝트 초기화
- [x] Go 모듈 초기화
- [x] 의존성 설치 (go-webview2, x/sys/windows)
- [x] 아이콘 생성 및 exe 임베딩

## Phase 2: 핵심 구현
- [x] WebView2 브라우저 창
- [x] 네이티브 탭바 (GDI 렌더링)
- [x] WebView2 bounds 조정 (탭바 아래 영역)
- [x] 시스템 트레이 (Win32 API)

## Phase 3: 탭 관리
- [x] 탭 클릭 → Navigate
- [x] 탭 우클릭 → 삭제 메뉴 (실수 방지)
- [x] ⚙ 설정 페이지 (프리셋 4개 AI + 커스텀 추가)
- [x] tabs.json 저장/로드

## Phase 4: 보호 기능
- [x] X 버튼 → 트레이 숨기기 (종료 방지)
- [x] 외부 링크 → 기본 브라우저에서 열기 (탭 덮어쓰기 방지)
- [x] 중복실행 방지 (Mutex)
- [x] 단독 exe 빌드 (-H windowsgui)
