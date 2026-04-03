use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;
use std::sync::{Mutex, atomic::{AtomicBool, Ordering}};
#[cfg(windows)]
use std::os::windows::process::CommandExt;
use tauri::{
    menu::{MenuBuilder, MenuItemBuilder},
    tray::TrayIconBuilder,
    webview::WebviewBuilder,
    Manager, State, LogicalPosition, LogicalSize,
};

const TABBAR_H: f64 = 48.0;
const APP_VERSION: &str = env!("CARGO_PKG_VERSION");

// 탭 설정
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TabItem {
    pub name: String,
    pub url: String,
    pub color: String,
}

// 앱 상태
pub struct AppState {
    pub tabs: Mutex<Vec<TabItem>>,
    pub active_tab: Mutex<usize>,
    pub config_path: PathBuf,
}

// 기본 탭
fn default_tabs() -> Vec<TabItem> {
    vec![
        TabItem { name: "Claude".into(), url: "https://claude.ai".into(), color: "#D2A032".into() },
        TabItem { name: "Gemini".into(), url: "https://gemini.google.com".into(), color: "#4285F4".into() },
        TabItem { name: "ChatGPT".into(), url: "https://chatgpt.com".into(), color: "#10A37F".into() },
        TabItem { name: "Grok".into(), url: "https://grok.com".into(), color: "#8C64FF".into() },
    ]
}

// 프리셋 목록
#[tauri::command]
fn get_presets() -> Vec<TabItem> {
    vec![
        TabItem { name: "Claude".into(), url: "https://claude.ai".into(), color: "#D2A032".into() },
        TabItem { name: "Gemini".into(), url: "https://gemini.google.com".into(), color: "#4285F4".into() },
        TabItem { name: "ChatGPT".into(), url: "https://chatgpt.com".into(), color: "#10A37F".into() },
        TabItem { name: "Grok".into(), url: "https://grok.com".into(), color: "#8C64FF".into() },
        TabItem { name: "Perplexity".into(), url: "https://perplexity.ai".into(), color: "#20A8E8".into() },
        TabItem { name: "Copilot".into(), url: "https://copilot.microsoft.com".into(), color: "#44B6E8".into() },
        TabItem { name: "NotebookLM".into(), url: "https://notebooklm.google.com".into(), color: "#F5A242".into() },
        TabItem { name: "AI Studio".into(), url: "https://aistudio.google.com".into(), color: "#4285F4".into() },
        TabItem { name: "Poe".into(), url: "https://poe.com".into(), color: "#5599CC".into() },
        TabItem { name: "HuggingChat".into(), url: "https://huggingface.co/chat".into(), color: "#FFD400".into() },
        TabItem { name: "DeepSeek".into(), url: "https://chat.deepseek.com".into(), color: "#4B8CFF".into() },
        TabItem { name: "Mistral".into(), url: "https://chat.mistral.ai".into(), color: "#FFAC2B".into() },
    ]
}

fn get_config_path() -> PathBuf {
    let app_data = dirs::config_dir().unwrap_or_else(|| PathBuf::from("."));
    let dir = app_data.join("AIBrowser");
    fs::create_dir_all(&dir).ok();
    dir.join("tabs.json")
}

fn load_tabs(path: &PathBuf) -> Vec<TabItem> {
    if let Ok(data) = fs::read_to_string(path) {
        if let Ok(tabs) = serde_json::from_str::<Vec<TabItem>>(&data) {
            if !tabs.is_empty() {
                return tabs;
            }
        }
    }
    default_tabs()
}

fn save_tabs(path: &PathBuf, tabs: &[TabItem]) {
    if let Ok(data) = serde_json::to_string_pretty(tabs) {
        fs::write(path, data).ok();
    }
}

// 설정 패널 열기/닫기 시 메인 웹뷰 크기 조정
#[tauri::command]
fn toggle_settings_view(app: tauri::AppHandle, state: State<AppState>, open: bool) {
    let (tab_count, active) = {
        let tabs = state.tabs.lock().unwrap();
        let active = *state.active_tab.lock().unwrap();
        (tabs.len(), active)
    }; // 락 해제 후 UI 조작 (데드락 방지)

    if let Some(win) = app.get_window("main") {
        let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
        let scale = win.scale_factor().unwrap_or(1.0);
        let w = size.width as f64 / scale;
        let h = size.height as f64 / scale;

        if let Some(main_wv) = app.get_webview("main") {
            if open {
                main_wv.set_size(LogicalSize::new(w, h)).ok();
            } else {
                main_wv.set_size(LogicalSize::new(w, TABBAR_H)).ok();
            }
        }

        for i in 0..tab_count {
            let label = format!("tab-{}", i);
            if let Some(wv) = app.get_webview(&label) {
                if !open && i == active {
                    wv.set_position(LogicalPosition::new(0.0, TABBAR_H)).ok();
                    wv.set_size(LogicalSize::new(w, h - TABBAR_H)).ok();
                } else {
                    wv.set_position(LogicalPosition::new(-10000.0, -10000.0)).ok();
                    wv.set_size(LogicalSize::new(0.0, 0.0)).ok();
                }
            }
        }
    }
}

#[tauri::command]
fn get_tabs(state: State<AppState>) -> Vec<TabItem> {
    state.tabs.lock().unwrap().clone()
}

#[tauri::command]
fn get_active_tab(state: State<AppState>) -> usize {
    *state.active_tab.lock().unwrap()
}

// 웹뷰 뒤로 가기
#[tauri::command]
fn go_back(app: tauri::AppHandle, state: State<AppState>) {
    let active = *state.active_tab.lock().unwrap();
    let label = format!("tab-{}", active);
    if let Some(wv) = app.get_webview(&label) {
        wv.eval("window.history.back()").ok();
    }
}

// 웹뷰 앞으로 가기
#[tauri::command]
fn go_forward(app: tauri::AppHandle, state: State<AppState>) {
    let active = *state.active_tab.lock().unwrap();
    let label = format!("tab-{}", active);
    if let Some(wv) = app.get_webview(&label) {
        wv.eval("window.history.forward()").ok();
    }
}

// 현재 탭을 원래 URL로 이동
#[tauri::command]
fn go_home(app: tauri::AppHandle, state: State<AppState>) {
    let (active, url) = {
        let active = *state.active_tab.lock().unwrap();
        let tabs = state.tabs.lock().unwrap();
        let url = tabs.get(active).map(|t| t.url.clone());
        (active, url)
    };
    if let Some(url) = url {
        let label = format!("tab-{}", active);
        if let Some(wv) = app.get_webview(&label) {
            wv.eval(&format!("window.location.href = '{}'", url)).ok();
        }
    }
}

// 탭 전환: 해당 webview를 보이고 나머지 숨김
#[tauri::command]
fn switch_tab(app: tauri::AppHandle, state: State<AppState>, index: usize) {
    let tab_count = {
        let tabs = state.tabs.lock().unwrap();
        if index >= tabs.len() { return; }
        tabs.len()
    };
    *state.active_tab.lock().unwrap() = index;

    if let Some(win) = app.get_window("main") {
        let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
        let scale = win.scale_factor().unwrap_or(1.0);
        let w = size.width as f64 / scale;
        let h = size.height as f64 / scale;

        if let Some(main_wv) = app.get_webview("main") {
            main_wv.set_size(LogicalSize::new(w, TABBAR_H)).ok();
        }

        for i in 0..tab_count {
            let label = format!("tab-{}", i);
            if let Some(wv) = app.get_webview(&label) {
                if i == index {
                    wv.set_position(LogicalPosition::new(0.0, TABBAR_H)).ok();
                    wv.set_size(LogicalSize::new(w, h - TABBAR_H)).ok();
                } else {
                    wv.set_position(LogicalPosition::new(-10000.0, -10000.0)).ok();
                    wv.set_size(LogicalSize::new(0.0, 0.0)).ok();
                }
            }
        }
    }
}

#[tauri::command]
fn add_tab(app: tauri::AppHandle, state: State<AppState>, name: String, url: String, color: String) {
    let (idx, url) = {
        let mut tabs = state.tabs.lock().unwrap();
        let url = if url.contains("://") { url } else { format!("https://{}", url) };
        let idx = tabs.len();
        tabs.push(TabItem { name, url: url.clone(), color });
        save_tabs(&state.config_path, &tabs);
        (idx, url)
    }; // 락 해제 후 웹뷰 생성 (데드락 방지)

    if let Some(win) = app.get_window("main") {
        if let Ok(parsed) = url.parse() {
            let label = format!("tab-{}", idx);
            let same_tab_js = r#"
(function() {
    window.open = function(url) {
        if (url && url !== '' && url !== 'about:blank' && !url.startsWith('javascript:')) {
            window.location.href = url;
        }
        return window;
    };
    document.addEventListener('click', function(e) {
        var a = e.target.closest('a[target="_blank"]');
        if (a && a.href && a.href !== '#' && !a.href.startsWith('javascript:')) {
            e.preventDefault(); window.location.href = a.href;
        }
    }, true);
})();"#;
            let builder = WebviewBuilder::new(&label, tauri::WebviewUrl::External(parsed))
                .initialization_script(same_tab_js);
            match win.add_child(builder, LogicalPosition::new(-10000.0, -10000.0), LogicalSize::new(0.0, 0.0)) {
                Ok(_) => eprintln!("[add_tab] 웹뷰 {} 생성 성공", label),
                Err(e) => eprintln!("[add_tab] 웹뷰 {} 생성 실패: {:?}", label, e),
            }
        } else {
            eprintln!("[add_tab] URL 파싱 실패: {}", url);
        }
    } else {
        eprintln!("[add_tab] main 윈도우 없음");
    }
}

#[tauri::command]
fn remove_tab(state: State<AppState>, index: usize) -> bool {
    let mut tabs = state.tabs.lock().unwrap();
    if tabs.len() <= 1 || index >= tabs.len() { return false; }
    tabs.remove(index);
    save_tabs(&state.config_path, &tabs);
    true
}

#[tauri::command]
fn reorder_tab(state: State<AppState>, from: usize, to: usize) {
    let mut tabs = state.tabs.lock().unwrap();
    if from >= tabs.len() || to >= tabs.len() { return; }
    let tab = tabs.remove(from);
    tabs.insert(to, tab);
    save_tabs(&state.config_path, &tabs);
}

#[tauri::command]
fn get_version() -> String {
    APP_VERSION.to_string()
}

// 업데이트 확인: (버전, 릴리스페이지, 설치파일URL) 반환
#[tauri::command]
fn check_update() -> Option<(String, String, String)> {
    let resp = ureq::get("https://api.github.com/repos/ReachToWisdom/AI-Browser/releases/latest")
        .set("User-Agent", "AI-Browser")
        .call().ok()?;
    let json: serde_json::Value = resp.into_json().ok()?;
    let tag = json["tag_name"].as_str()?.trim_start_matches('v').to_string();
    let html_url = json["html_url"].as_str()?.to_string();

    // 설치파일(setup.exe) 에셋 URL 찾기
    let asset_url = json["assets"].as_array()
        .and_then(|assets| assets.iter()
            .find(|a| a["name"].as_str().map_or(false, |n| n.contains("setup")))
            .and_then(|a| a["browser_download_url"].as_str().map(|s| s.to_string()))
        )
        .unwrap_or_else(|| html_url.clone());

    if is_newer(&tag, APP_VERSION) { Some((tag, html_url, asset_url)) } else { None }
}

// 설치파일 다운로드 후 실행
#[tauri::command]
fn download_and_install(app: tauri::AppHandle, download_url: String) -> Result<(), String> {
    let timestamp = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH).unwrap_or_default().as_secs();
    let setup_path = std::env::temp_dir().join(format!("AI-Browser-setup-{}.exe", timestamp));
    let bat_path = std::env::temp_dir().join(format!("AI-Browser-update-{}.bat", timestamp));

    let resp = ureq::get(&download_url)
        .set("User-Agent", "AI-Browser")
        .call()
        .map_err(|e| format!("다운로드 실패: {}", e))?;
    let mut file = fs::File::create(&setup_path).map_err(|e| format!("파일 생성 실패: {}", e))?;
    std::io::copy(&mut resp.into_reader(), &mut file).map_err(|e| format!("저장 실패: {}", e))?;
    drop(file);

    // bat 스크립트: 2초 대기 후 인스톨러 실행 (파일 잠금 해제 대기)
    let script = format!(
        "@echo off\r\nping 127.0.0.1 -n 3 >nul\r\nstart \"\" \"{}\"\r\ndel \"%~f0\"\r\n",
        setup_path.to_string_lossy()
    );
    fs::write(&bat_path, &script).map_err(|e| format!("스크립트 생성 실패: {}", e))?;

    let mut cmd = std::process::Command::new(&bat_path);
    #[cfg(windows)]
    cmd.creation_flags(0x08000000); // CREATE_NO_WINDOW
    cmd.spawn().map_err(|e| format!("실행 실패: {}", e))?;

    app.exit(0);
    Ok(())
}

fn is_newer(latest: &str, current: &str) -> bool {
    let l: Vec<u32> = latest.split('.').filter_map(|s| s.parse().ok()).collect();
    let c: Vec<u32> = current.split('.').filter_map(|s| s.parse().ok()).collect();
    for i in 0..l.len().max(c.len()) {
        let lv = l.get(i).copied().unwrap_or(0);
        let cv = c.get(i).copied().unwrap_or(0);
        if lv > cv { return true; }
        if lv < cv { return false; }
    }
    false
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let config_path = get_config_path();
    let tabs = load_tabs(&config_path);
    let _tab_count = tabs.len();

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            // 중복 실행 시 기존 창 활성화
            if let Some(w) = app.get_window("main") {
                w.show().ok();
                w.set_focus().ok();
            }
        }))
        .plugin(tauri_plugin_opener::init())
        .manage(AppState {
            tabs: Mutex::new(tabs.clone()),
            active_tab: Mutex::new(0),
            config_path,
        })
        .setup(move |app| {
            // 실제 종료 플래그
            let quitting = std::sync::Arc::new(AtomicBool::new(false));
            let quitting_tray = quitting.clone();
            let quitting_close = quitting.clone();
            // 메인 윈도우 (탭바 UI용)
            let win = app.get_window("main").unwrap();

            // 메인 webview(탭바)를 상단 48px로 제한
            if let Some(main_wv) = app.get_webview("main") {
                main_wv.set_position(LogicalPosition::new(0.0, 0.0)).ok();
                main_wv.set_size(LogicalSize::new(1400.0, TABBAR_H)).ok();
            }

            let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
            let scale = win.scale_factor().unwrap_or(1.0);
            let w = size.width as f64 / scale;
            let h = size.height as f64 / scale;

            // 새 창 요청을 같은 탭에서 열기
            let same_tab_script = r#"
(function() {
    // window.open 단순 오버라이드
    window.open = function(url) {
        if (url && url !== '' && url !== 'about:blank' && !url.startsWith('javascript:')) {
            window.location.href = url;
        }
        return window;
    };
    // target="_blank" 클릭만 가로채기 (다른 이벤트는 건드리지 않음)
    document.addEventListener('click', function(e) {
        var a = e.target.closest('a[target="_blank"]');
        if (a && a.href && a.href !== '#' && !a.href.startsWith('javascript:')) {
            e.preventDefault();
            window.location.href = a.href;
        }
    }, true);
})();
"#;

            // 각 탭에 대해 webview 생성
            for (i, tab) in tabs.iter().enumerate() {
                let label = format!("tab-{}", i);
                let url: tauri::WebviewUrl = tauri::WebviewUrl::External(tab.url.parse().unwrap());
                let builder = WebviewBuilder::new(&label, url)
                    .initialization_script(same_tab_script);

                if i == 0 {
                    win.add_child(builder, LogicalPosition::new(0.0, TABBAR_H), LogicalSize::new(w, h - TABBAR_H)).ok();
                } else {
                    win.add_child(builder, LogicalPosition::new(-10000.0, -10000.0), LogicalSize::new(0.0, 0.0)).ok();
                }
            }

            // 시스템 트레이
            let show = MenuItemBuilder::with_id("show", "열기").build(app)?;
            let about = MenuItemBuilder::with_id("about", "프로그램 정보").build(app)?;
            let quit = MenuItemBuilder::with_id("quit", "종료").build(app)?;
            let menu = MenuBuilder::new(app)
                .item(&show).separator().item(&about).separator().item(&quit)
                .build()?;

            let _tray = TrayIconBuilder::new()
                .icon(app.default_window_icon().cloned().unwrap())
                .menu(&menu)
                .tooltip("AI Browser")
                .on_menu_event(move |app, event| {
                    match event.id().as_ref() {
                        "show" => {
                            if let Some(w) = app.get_window("main") {
                                w.show().ok(); w.set_focus().ok();
                            }
                        }
                        "about" => {
                            if let Some(wv) = app.get_webview_window("main") {
                                wv.eval(&format!("alert('AI Browser v{}\\n\\n개발자: 혜통')", APP_VERSION)).ok();
                            }
                        }
                        "quit" => {
                            quitting_tray.store(true, Ordering::SeqCst);
                            if let Some(w) = app.get_window("main") {
                                w.destroy().ok();
                            }
                            app.exit(0);
                            std::process::exit(0);
                        }
                        _ => {}
                    }
                })
                .on_tray_icon_event(|tray, event| {
                    if let tauri::tray::TrayIconEvent::DoubleClick { .. } = event {
                        if let Some(w) = tray.app_handle().get_window("main") {
                            w.show().ok(); w.set_focus().ok();
                        }
                    }
                })
                .build(app)?;

            // 닫기 → 트레이 (종료 플래그가 아닌 경우만)
            let window = app.get_window("main").unwrap();
            let w2 = window.clone();
            window.on_window_event(move |event| {
                if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                    if !quitting_close.load(Ordering::SeqCst) {
                        api.prevent_close();
                        w2.hide().ok();
                    }
                }
            });

            // 윈도우 리사이즈 시 활성 webview 크기 조정
            let app_handle = app.handle().clone();
            let win2 = app.get_window("main").unwrap();
            win2.on_window_event(move |event| {
                if let tauri::WindowEvent::Resized(_) = event {
                    let state: State<AppState> = app_handle.state();
                    // try_lock으로 데드락 방지 (다른 곳에서 락 잡고 있으면 스킵)
                    let active = match state.active_tab.try_lock() {
                        Ok(a) => *a,
                        Err(_) => return,
                    };
                    let tab_count = match state.tabs.try_lock() {
                        Ok(t) => t.len(),
                        Err(_) => return,
                    };

                    if let Some(win) = app_handle.get_window("main") {
                        let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
                        let scale = win.scale_factor().unwrap_or(1.0);
                        let w = size.width as f64 / scale;
                        let h = size.height as f64 / scale;

                        if let Some(main_wv) = app_handle.get_webview("main") {
                            main_wv.set_size(LogicalSize::new(w, TABBAR_H)).ok();
                        }

                        for i in 0..tab_count {
                            let label = format!("tab-{}", i);
                            if let Some(wv) = app_handle.get_webview(&label) {
                                if i == active {
                                    wv.set_position(LogicalPosition::new(0.0, TABBAR_H)).ok();
                                    wv.set_size(LogicalSize::new(w, h - TABBAR_H)).ok();
                                }
                            }
                        }
                    }
                }
            });

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            get_tabs, get_active_tab, switch_tab,
            add_tab, remove_tab, reorder_tab,
            get_presets, check_update, toggle_settings_view,
            go_back, go_forward, go_home, download_and_install, get_version,
        ])
        .run(tauri::generate_context!())
        .expect("AI Browser 실행 실패");
}
