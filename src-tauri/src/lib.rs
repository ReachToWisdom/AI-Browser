use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;
use std::sync::{Mutex, atomic::{AtomicBool, AtomicU64, Ordering}};
#[cfg(windows)]
use std::os::windows::process::CommandExt;
use tauri::{
    menu::{MenuBuilder, MenuItemBuilder},
    tray::TrayIconBuilder,
    webview::WebviewBuilder,
    Manager, State, LogicalPosition, LogicalSize,
};

const TABBAR_H: f64 = 48.0;
// macOS 네이티브 타이틀바 높이 — 자식 WebView 좌표가 NSWindow 프레임 기준이라 오프셋 필요
#[cfg(target_os = "macos")]
const TITLEBAR_H: f64 = 28.0;
#[cfg(not(target_os = "macos"))]
const TITLEBAR_H: f64 = 0.0;
const APP_VERSION: &str = env!("CARGO_PKG_VERSION");

// 탭 설정 (id 필드 추가, 기존 config 호환)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TabItem {
    pub name: String,
    pub url: String,
    pub color: String,
    #[serde(default)]
    pub id: String,
}

// 앱 상태
pub struct AppState {
    pub tabs: Mutex<Vec<TabItem>>,
    pub active_tab: Mutex<usize>,
    pub config_path: PathBuf,
    pub overlay_open: AtomicBool,
    pub next_id: AtomicU64,
}

fn gen_id(counter: &AtomicU64) -> String {
    format!("tab-{}", counter.fetch_add(1, Ordering::SeqCst))
}

// 기본 탭
fn default_tabs(counter: &AtomicU64) -> Vec<TabItem> {
    vec![
        TabItem { name: "Claude".into(), url: "https://claude.ai".into(), color: "#D2A032".into(), id: gen_id(counter) },
        TabItem { name: "Gemini".into(), url: "https://gemini.google.com".into(), color: "#4285F4".into(), id: gen_id(counter) },
        TabItem { name: "ChatGPT".into(), url: "https://chatgpt.com".into(), color: "#10A37F".into(), id: gen_id(counter) },
        TabItem { name: "Grok".into(), url: "https://grok.com".into(), color: "#8C64FF".into(), id: gen_id(counter) },
    ]
}

// 프리셋 목록
#[tauri::command]
fn get_presets() -> Vec<TabItem> {
    vec![
        TabItem { name: "Claude".into(), url: "https://claude.ai".into(), color: "#D2A032".into(), id: String::new() },
        TabItem { name: "Gemini".into(), url: "https://gemini.google.com".into(), color: "#4285F4".into(), id: String::new() },
        TabItem { name: "ChatGPT".into(), url: "https://chatgpt.com".into(), color: "#10A37F".into(), id: String::new() },
        TabItem { name: "Grok".into(), url: "https://grok.com".into(), color: "#8C64FF".into(), id: String::new() },
        TabItem { name: "Perplexity".into(), url: "https://perplexity.ai".into(), color: "#20A8E8".into(), id: String::new() },
        TabItem { name: "Copilot".into(), url: "https://copilot.microsoft.com".into(), color: "#44B6E8".into(), id: String::new() },
        TabItem { name: "NotebookLM".into(), url: "https://notebooklm.google.com".into(), color: "#F5A242".into(), id: String::new() },
        TabItem { name: "AI Studio".into(), url: "https://aistudio.google.com".into(), color: "#4285F4".into(), id: String::new() },
        TabItem { name: "Poe".into(), url: "https://poe.com".into(), color: "#5599CC".into(), id: String::new() },
        TabItem { name: "HuggingChat".into(), url: "https://huggingface.co/chat".into(), color: "#FFD400".into(), id: String::new() },
        TabItem { name: "DeepSeek".into(), url: "https://chat.deepseek.com".into(), color: "#4B8CFF".into(), id: String::new() },
        TabItem { name: "Mistral".into(), url: "https://chat.mistral.ai".into(), color: "#FFAC2B".into(), id: String::new() },
        TabItem { name: "Genspark".into(), url: "https://www.genspark.ai".into(), color: "#6C5CE7".into(), id: String::new() },
    ]
}

fn get_config_path() -> PathBuf {
    let app_data = dirs::config_dir().unwrap_or_else(|| PathBuf::from("."));
    let dir = app_data.join("AIBrowser");
    fs::create_dir_all(&dir).ok();
    dir.join("tabs.json")
}

fn load_tabs(path: &PathBuf, counter: &AtomicU64) -> Vec<TabItem> {
    if let Ok(data) = fs::read_to_string(path) {
        if let Ok(mut tabs) = serde_json::from_str::<Vec<TabItem>>(&data) {
            if !tabs.is_empty() {
                // 기존 config에 id가 없는 경우 자동 부여
                for tab in &mut tabs {
                    if tab.id.is_empty() {
                        tab.id = gen_id(counter);
                    }
                }
                // counter를 현재 최대값 이상으로 설정
                let max_id = tabs.iter()
                    .filter_map(|t| t.id.strip_prefix("tab-").and_then(|s| s.parse::<u64>().ok()))
                    .max().unwrap_or(0);
                let current = counter.load(Ordering::SeqCst);
                if max_id >= current {
                    counter.store(max_id + 1, Ordering::SeqCst);
                }
                return tabs;
            }
        }
    }
    default_tabs(counter)
}

fn save_tabs(path: &PathBuf, tabs: &[TabItem]) {
    if let Ok(data) = serde_json::to_string_pretty(tabs) {
        fs::write(path, data).ok();
    }
}

// 활성 탭의 id 가져오기
fn get_active_id(state: &AppState) -> Option<String> {
    let active = *state.active_tab.lock().unwrap();
    let tabs = state.tabs.lock().unwrap();
    tabs.get(active).map(|t| t.id.clone())
}

// 모든 탭 id 목록 가져오기
fn get_all_ids(state: &AppState) -> Vec<String> {
    state.tabs.lock().unwrap().iter().map(|t| t.id.clone()).collect()
}

// 설정/모달 열기/닫기
#[tauri::command]
fn toggle_settings_view(app: tauri::AppHandle, state: State<AppState>, open: bool) {
    state.overlay_open.store(open, Ordering::SeqCst);
    let active_id = get_active_id(&state);

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

        let all_tabs: Vec<TabItem> = state.tabs.lock().unwrap().clone();
        for tab in &all_tabs {
            let is_active = !open && active_id.as_deref() == Some(tab.id.as_str());
            if is_active {
                // 활성 탭 웹뷰 ensure 후 직접 참조 사용
                if let Some(wv) = ensure_webview(&app, &win, tab) {
                    wv.set_position(LogicalPosition::new(0.0, TITLEBAR_H + TABBAR_H)).ok();
                    wv.set_size(LogicalSize::new(w, h - TABBAR_H)).ok();
                }
            } else {
                if let Some(wv) = app.get_webview(&tab.id) {
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

#[tauri::command]
fn go_back(app: tauri::AppHandle, state: State<AppState>) {
    if let Some(id) = get_active_id(&state) {
        if let Some(wv) = app.get_webview(&id) {
            wv.eval("window.history.back()").ok();
        }
    }
}

#[tauri::command]
fn go_forward(app: tauri::AppHandle, state: State<AppState>) {
    if let Some(id) = get_active_id(&state) {
        if let Some(wv) = app.get_webview(&id) {
            wv.eval("window.history.forward()").ok();
        }
    }
}

#[tauri::command]
fn go_home(app: tauri::AppHandle, state: State<AppState>) {
    let (id, url) = {
        let active = *state.active_tab.lock().unwrap();
        let tabs = state.tabs.lock().unwrap();
        tabs.get(active).map(|t| (t.id.clone(), t.url.clone())).unwrap_or_default()
    };
    if !id.is_empty() {
        if let Some(wv) = app.get_webview(&id) {
            let safe_url = url.replace('\'', "%27");
            wv.eval(&format!("window.location.href = '{}'", safe_url)).ok();
        }
    }
}

// 탭 전환 (lazy webview 생성 포함)
#[tauri::command]
fn switch_tab(app: tauri::AppHandle, state: State<AppState>, index: usize) {
    let (target_tab, all_tabs) = {
        let tabs = state.tabs.lock().unwrap();
        if index >= tabs.len() { return; }
        (tabs[index].clone(), tabs.clone())
    };
    *state.active_tab.lock().unwrap() = index;

    if let Some(win) = app.get_window("main") {
        // 대상 탭 웹뷰가 없으면 생성 (반환값 직접 사용)
        let target_wv = ensure_webview(&app, &win, &target_tab);

        let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
        let scale = win.scale_factor().unwrap_or(1.0);
        let w = size.width as f64 / scale;
        let h = size.height as f64 / scale;

        if let Some(main_wv) = app.get_webview("main") {
            main_wv.set_size(LogicalSize::new(w, TABBAR_H)).ok();
        }

        // 대상 탭 웹뷰 표시 (ensure_webview에서 직접 받은 참조 사용)
        if let Some(wv) = target_wv {
            wv.set_position(LogicalPosition::new(0.0, TITLEBAR_H + TABBAR_H)).ok();
            wv.set_size(LogicalSize::new(w, h - TABBAR_H)).ok();
        }

        // 나머지 탭 숨김
        for tab in &all_tabs {
            if tab.id != target_tab.id {
                if let Some(wv) = app.get_webview(&tab.id) {
                    wv.set_position(LogicalPosition::new(-10000.0, -10000.0)).ok();
                    wv.set_size(LogicalSize::new(0.0, 0.0)).ok();
                }
            }
        }
    }
}

fn same_tab_script() -> &'static str {
    r#"(function() {
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
})();"#
}

// 탭 추가 (상태만 저장, 웹뷰는 switch_tab에서 lazy 생성)
#[tauri::command]
fn add_tab(state: State<AppState>, name: String, url: String, color: String) -> usize {
    let mut tabs = state.tabs.lock().unwrap();
    let url = if url.contains("://") { url } else { format!("https://{}", url) };
    let id = gen_id(&state.next_id);
    let new_idx = tabs.len();
    tabs.push(TabItem { name, url: url.clone(), color, id: id.clone() });
    save_tabs(&state.config_path, &tabs);
    new_idx
}

// 웹뷰가 없으면 생성, 생성된 웹뷰 반환
fn ensure_webview(app: &tauri::AppHandle, win: &tauri::Window, tab: &TabItem) -> Option<tauri::Webview> {
    if let Some(wv) = app.get_webview(&tab.id) { return Some(wv); }
    if let Ok(parsed) = tab.url.parse::<tauri::Url>() {
        let builder = WebviewBuilder::new(&tab.id, tauri::WebviewUrl::External(parsed))
            .initialization_script(same_tab_script());
        match win.add_child(builder, LogicalPosition::new(-10000.0, -10000.0), LogicalSize::new(0.0, 0.0)) {
            Ok(wv) => Some(wv),
            Err(_) => None,
        }
    } else {
        None
    }
}

#[tauri::command]
fn remove_tab(app: tauri::AppHandle, state: State<AppState>, index: usize) -> bool {
    let removed_id = {
        let mut tabs = state.tabs.lock().unwrap();
        if tabs.len() <= 1 || index >= tabs.len() { return false; }
        let removed = tabs.remove(index);
        save_tabs(&state.config_path, &tabs);
        // active_tab 조정
        let mut active = state.active_tab.lock().unwrap();
        if *active >= tabs.len() {
            *active = tabs.len() - 1;
        }
        removed.id
    };

    // 제거된 웹뷰 숨김 (Tauri 2 child webview는 destroy 불가)
    if let Some(wv) = app.get_webview(&removed_id) {
        wv.set_position(LogicalPosition::new(-10000.0, -10000.0)).ok();
        wv.set_size(LogicalSize::new(0.0, 0.0)).ok();
    }
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
fn restart_app(app: tauri::AppHandle) {
    app.restart();
}

// 설정에서 일괄 저장: 탭 목록 교체 (id 비어있으면 신규 발급, 제거된 웹뷰 숨김)
#[tauri::command]
fn replace_tabs(app: tauri::AppHandle, state: State<AppState>, new_tabs: Vec<TabItem>) -> Vec<TabItem> {
    let old_ids: Vec<String> = {
        let tabs = state.tabs.lock().unwrap();
        tabs.iter().map(|t| t.id.clone()).collect()
    };

    // 빈 id에 신규 발급
    let mut processed: Vec<TabItem> = Vec::with_capacity(new_tabs.len());
    for mut t in new_tabs {
        if t.id.is_empty() {
            t.id = gen_id(&state.next_id);
        }
        if !t.url.contains("://") {
            t.url = format!("https://{}", t.url);
        }
        processed.push(t);
    }

    // 제거된 탭 웹뷰 숨김
    let new_id_set: std::collections::HashSet<String> = processed.iter().map(|t| t.id.clone()).collect();
    for old_id in &old_ids {
        if !new_id_set.contains(old_id) {
            if let Some(wv) = app.get_webview(old_id) {
                wv.set_position(LogicalPosition::new(-10000.0, -10000.0)).ok();
                wv.set_size(LogicalSize::new(0.0, 0.0)).ok();
            }
        }
    }

    // 상태/디스크 갱신
    {
        let mut tabs = state.tabs.lock().unwrap();
        *tabs = processed.clone();
        save_tabs(&state.config_path, &tabs);
    }
    {
        let tabs = state.tabs.lock().unwrap();
        let mut active = state.active_tab.lock().unwrap();
        if *active >= tabs.len() {
            *active = tabs.len().saturating_sub(1);
        }
    }

    processed
}

#[tauri::command]
fn get_version() -> String {
    APP_VERSION.to_string()
}

#[tauri::command]
fn check_update() -> Option<(String, String, String)> {
    let resp = ureq::get("https://api.github.com/repos/ReachToWisdom/AI-Browser/releases/latest")
        .set("User-Agent", "AI-Browser")
        .call().ok()?;
    let json: serde_json::Value = resp.into_json().ok()?;
    let tag = json["tag_name"].as_str()?.trim_start_matches('v').to_string();
    let html_url = json["html_url"].as_str()?.to_string();
    let asset_url = json["assets"].as_array()
        .and_then(|assets| assets.iter()
            .find(|a| a["name"].as_str().map_or(false, |n| n.contains("setup")))
            .and_then(|a| a["browser_download_url"].as_str().map(|s| s.to_string()))
        )
        .unwrap_or_else(|| html_url.clone());
    if is_newer(&tag, APP_VERSION) { Some((tag, html_url, asset_url)) } else { None }
}

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

    let script = format!(
        "@echo off\r\nping 127.0.0.1 -n 3 >nul\r\nstart \"\" \"{}\"\r\ndel \"%~f0\"\r\n",
        setup_path.to_string_lossy()
    );
    fs::write(&bat_path, &script).map_err(|e| format!("스크립트 생성 실패: {}", e))?;

    let mut cmd = std::process::Command::new(&bat_path);
    #[cfg(windows)]
    cmd.creation_flags(0x08000000);
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
    let counter = AtomicU64::new(0);
    let tabs = load_tabs(&config_path, &counter);
    let initial_next_id = counter.load(Ordering::SeqCst);

    // id가 새로 부여된 경우 저장
    save_tabs(&config_path, &tabs);

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
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
            overlay_open: AtomicBool::new(false),
            next_id: AtomicU64::new(initial_next_id),
        })
        .setup(move |app| {
            let quitting = std::sync::Arc::new(AtomicBool::new(false));
            let quitting_tray = quitting.clone();
            let quitting_close = quitting.clone();
            let win = app.get_window("main").unwrap();

            if let Some(main_wv) = app.get_webview("main") {
                main_wv.set_position(LogicalPosition::new(0.0, TITLEBAR_H)).ok();
                main_wv.set_size(LogicalSize::new(1400.0, TABBAR_H)).ok();
            }

            let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
            let scale = win.scale_factor().unwrap_or(1.0);
            let w = size.width as f64 / scale;
            let h = size.height as f64 / scale;

            // 각 탭에 대해 webview 생성 (id 기반 라벨)
            let first_tab_id = tabs.first().map(|t| t.id.clone());
            for (i, tab) in tabs.iter().enumerate() {
                let url: tauri::WebviewUrl = tauri::WebviewUrl::External(tab.url.parse().unwrap());
                let builder = WebviewBuilder::new(&tab.id, url)
                    .initialization_script(same_tab_script());

                if i == 0 {
                    win.add_child(builder, LogicalPosition::new(0.0, TITLEBAR_H + TABBAR_H), LogicalSize::new(w, h - TABBAR_H)).ok();
                } else {
                    win.add_child(builder, LogicalPosition::new(-10000.0, -10000.0), LogicalSize::new(0.0, 0.0)).ok();
                }
            }

            // macOS: 초기 inner_size 미실현으로 첫 탭이 공백 표시되는 문제 회피
            // 창이 실제로 표시된 후 첫 탭 위치/크기 재적용
            if let Some(first_id) = first_tab_id {
                let app_handle_init = app.handle().clone();
                tauri::async_runtime::spawn(async move {
                    tauri::async_runtime::spawn_blocking(|| {
                        std::thread::sleep(std::time::Duration::from_millis(300));
                    }).await.ok();
                    if let Some(win) = app_handle_init.get_window("main") {
                        let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
                        let scale = win.scale_factor().unwrap_or(1.0);
                        let w = size.width as f64 / scale;
                        let h = size.height as f64 / scale;
                        if let Some(main_wv) = app_handle_init.get_webview("main") {
                            main_wv.set_position(LogicalPosition::new(0.0, TITLEBAR_H)).ok();
                            main_wv.set_size(LogicalSize::new(w, TABBAR_H)).ok();
                        }
                        if let Some(wv) = app_handle_init.get_webview(&first_id) {
                            wv.set_position(LogicalPosition::new(0.0, TITLEBAR_H + TABBAR_H)).ok();
                            wv.set_size(LogicalSize::new(w, h - TABBAR_H)).ok();
                            wv.eval("location.reload()").ok();
                        }
                    }
                });
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
                                wv.eval(&format!("alert('AI Browser v{}\\n\\n개발자: 정성광')", APP_VERSION)).ok();
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

            // 닫기 → 트레이
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

                    // 오버레이 열림 상태면 메인 웹뷰 전체 유지
                    if state.overlay_open.load(Ordering::SeqCst) {
                        if let Some(win) = app_handle.get_window("main") {
                            let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
                            let scale = win.scale_factor().unwrap_or(1.0);
                            let w = size.width as f64 / scale;
                            let h = size.height as f64 / scale;
                            if let Some(main_wv) = app_handle.get_webview("main") {
                                main_wv.set_size(LogicalSize::new(w, h)).ok();
                            }
                        }
                        return;
                    }

                    let active_id = get_active_id(&state);
                    let all_ids = get_all_ids(&state);

                    if let Some(win) = app_handle.get_window("main") {
                        let size = win.inner_size().unwrap_or(tauri::PhysicalSize { width: 1400, height: 900 });
                        let scale = win.scale_factor().unwrap_or(1.0);
                        let w = size.width as f64 / scale;
                        let h = size.height as f64 / scale;

                        if let Some(main_wv) = app_handle.get_webview("main") {
                            main_wv.set_size(LogicalSize::new(w, TABBAR_H)).ok();
                        }

                        for id in &all_ids {
                            if let Some(wv) = app_handle.get_webview(id) {
                                if active_id.as_deref() == Some(id.as_str()) {
                                    wv.set_position(LogicalPosition::new(0.0, TITLEBAR_H + TABBAR_H)).ok();
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
            add_tab, remove_tab, reorder_tab, replace_tabs,
            get_presets, check_update, toggle_settings_view,
            go_back, go_forward, go_home, download_and_install, get_version, restart_app,
        ])
        .run(tauri::generate_context!())
        .expect("AI Browser 실행 실패");
}
