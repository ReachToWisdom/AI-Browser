const { invoke } = window.__TAURI__.core;

let tabs = [];
let activeTab = 0;
let settingsOpen = false;
// 설정 화면 진입 시 초기화되는 임시 버퍼 — 저장 클릭 시에만 커밋
let pendingTabs = null;
let pendingDirty = false;

// 메일 등 서브도메인 → 루트 도메인 favicon 매핑
const FAVICON_HOST_MAP = {
  "mail.naver.com": "www.naver.com",
  "mail.daum.net": "daum.net",
};
function faviconUrl(url) {
  try {
    const host = new URL(url).hostname;
    const domain = FAVICON_HOST_MAP[host] || host;
    return `https://www.google.com/s2/favicons?domain=${domain}&sz=32`;
  } catch {
    return "";
  }
}

window.addEventListener("DOMContentLoaded", async () => {
  tabs = await invoke("get_tabs");
  activeTab = await invoke("get_active_tab");
  renderTabBar();
  // 탭바 마우스 휠 가로 스크롤
  document.getElementById("tabbar").addEventListener("wheel", (e) => {
    e.preventDefault();
    const sa = document.getElementById("tab-scroll-area");
    if (sa) sa.scrollLeft += e.deltaY;
  }, { passive: false });
  setTimeout(checkUpdate, 1000);
});

// 탭바 렌더링
function renderTabBar() {
  const bar = document.getElementById("tabbar");
  let html = '<button class="tab-btn" id="btn-back" title="뒤로">◀</button>';
  html += '<button class="tab-btn" id="btn-forward" title="앞으로">▶</button>';
  html += '<button class="tab-btn" id="btn-refresh" title="새로고침">↻</button>';
  html += '<div id="tab-scroll-area">';
  tabs.forEach((tab, i) => {
    const favicon = faviconUrl(tab.url);
    html += `<button class="tab${i === activeTab && !settingsOpen ? ' active' : ''}" data-idx="${i}">
      <img class="tab-icon" src="${favicon}" onerror="this.style.display='none';this.nextElementSibling.style.display='block'" />
      <span class="dot" style="background:${tab.color};display:none"></span>
      <span class="name">${tab.name}</span>
    </button>`;
  });
  html += '</div>';
  html += '<button class="tab-btn tab-scroll-btn" id="btn-scroll-left" title="탭 왼쪽 스크롤">◁</button>';
  html += '<button class="tab-btn tab-scroll-btn" id="btn-scroll-right" title="탭 오른쪽 스크롤">▷</button>';
  html += '<button class="tab-btn" id="btn-settings" title="설정">⚙</button>';
  bar.innerHTML = html;

  bar.querySelectorAll(".tab").forEach(btn => {
    btn.addEventListener("click", () => {
      const idx = parseInt(btn.dataset.idx);
      if (idx === activeTab && !settingsOpen) {
        // 현재 탭 클릭 → 원래 URL로 이동
        invoke("go_home");
      } else {
        doSwitchTab(idx);
      }
    });
  });
  // 활성 탭이 보이도록 자동 스크롤
  const scrollArea = document.getElementById("tab-scroll-area");
  const activeBtn = scrollArea.querySelector(".tab.active");
  if (activeBtn) activeBtn.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "nearest" });

  // 탭 스크롤 버튼 (탭 1개 단위)
  document.getElementById("btn-scroll-left").addEventListener("click", () => {
    const tabBtn = scrollArea.querySelector(".tab");
    const step = tabBtn ? tabBtn.offsetWidth + 6 : 120;
    scrollArea.scrollLeft -= step;
  });
  document.getElementById("btn-scroll-right").addEventListener("click", () => {
    const tabBtn = scrollArea.querySelector(".tab");
    const step = tabBtn ? tabBtn.offsetWidth + 6 : 120;
    scrollArea.scrollLeft += step;
  });

  document.getElementById("btn-back").addEventListener("click", () => invoke("go_back"));
  document.getElementById("btn-forward").addEventListener("click", () => invoke("go_forward"));
  document.getElementById("btn-refresh").addEventListener("click", refreshCurrentTab);
  document.getElementById("btn-settings").addEventListener("click", toggleSettings);
}

// 탭 전환 (Rust 측 webview 전환)
async function doSwitchTab(idx) {
  if (settingsOpen) {
    settingsOpen = false;
    document.getElementById("settings").classList.add("hidden");
    await invoke("toggle_settings_view", { open: false });
  }
  activeTab = idx;
  await invoke("switch_tab", { index: idx });
  renderTabBar();
}

// 새로고침 - 현재는 탭 재전환으로 처리
function refreshCurrentTab() {
  if (settingsOpen) { toggleSettings(); return; }
  // Rust에서 webview reload은 별도 command 필요 → 추후 추가
}

// 설정 토글
async function toggleSettings() {
  if (settingsOpen && pendingDirty) {
    if (!confirm("저장하지 않은 변경사항이 있습니다. 버릴까요?")) return;
  }
  settingsOpen = !settingsOpen;
  if (settingsOpen) {
    // 열 때 현재 탭의 스냅샷으로 버퍼 초기화
    pendingTabs = tabs.map(t => ({ ...t }));
    pendingDirty = false;
  } else {
    pendingTabs = null;
    pendingDirty = false;
  }
  document.getElementById("settings").classList.toggle("hidden", !settingsOpen);
  await invoke("toggle_settings_view", { open: settingsOpen });
  if (settingsOpen) renderSettings();
  renderTabBar();
}

// 변경사항 커밋 (저장 버튼)
async function commitPendingTabs() {
  if (!pendingTabs) return;
  const saved = await invoke("replace_tabs", { newTabs: pendingTabs });
  tabs = saved;
  if (activeTab >= tabs.length) activeTab = Math.max(0, tabs.length - 1);
  pendingTabs = null;
  pendingDirty = false;

  // 저장 후 앱 재시작 (새 탭이 정상적으로 로드되도록)
  await invoke("restart_app");
}

// 설정 페이지
async function renderSettings() {
  const presets = await invoke("get_presets");

  let html = '<h2>📌 AI 프리셋 (원클릭 추가)</h2><div id="preset-list"></div>';
  html += '<h2>📋 현재 탭 <span id="dirty-flag" style="color:#f9e2af;font-size:12px;margin-left:8px;display:none">● 저장되지 않음</span></h2><div id="tab-list"></div>';
  html += '<p style="color:#585b70;font-size:11px;margin-top:6px">▲▼ 화살표로 순서 변경. 최소 1개 유지.</p>';
  html += '<h2>➕ 직접 추가</h2>';
  html += '<div class="form"><input id="add-name" placeholder="이름"><input id="add-url" placeholder="https://..."><button class="btn-add" id="btn-do-add">추가</button></div>';
  html += '<div class="form" style="margin-top:16px;gap:8px"><button class="btn-add" id="btn-save" style="flex:1;background:#a6e3a1;color:#1e1e2e;font-weight:600">💾 저장</button><button class="back-btn" id="btn-back-settings" style="flex:1">← 취소</button></div>';
  html += `<div class="about">AI Browser v${await getVersion()} · 개발자: 정성광</div>`;
  document.getElementById("settings").innerHTML = html;

  renderPresetList(presets);
  renderTabList();

  document.getElementById("preset-list").addEventListener("click", (e) => {
    const btn = e.target.closest(".btn-add:not([disabled])");
    if (!btn) return;
    pendingTabs.push({ name: btn.dataset.name, url: btn.dataset.url, color: btn.dataset.color, id: "" });
    markDirty();
    renderPresetList(presets);
    renderTabList();
  });

  function doAddTab() {
    const name = document.getElementById("add-name").value.trim();
    const url = document.getElementById("add-url").value.trim();
    if (!name || !url) { alert("이름과 URL을 입력하세요"); return; }
    pendingTabs.push({ name, url, color: "#888888", id: "" });
    markDirty();
    document.getElementById("add-name").value = "";
    document.getElementById("add-url").value = "";
    renderPresetList(presets);
    renderTabList();
  }

  document.getElementById("btn-do-add").addEventListener("click", doAddTab);
  document.getElementById("add-url").addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); doAddTab(); }
  });
  document.getElementById("add-name").addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); document.getElementById("add-url").focus(); }
  });

  document.getElementById("btn-save").addEventListener("click", commitPendingTabs);
  document.getElementById("btn-back-settings").addEventListener("click", toggleSettings);
}

function markDirty() {
  pendingDirty = true;
  const flag = document.getElementById("dirty-flag");
  if (flag) flag.style.display = "inline";
}

function renderPresetList(presets) {
  document.getElementById("preset-list").innerHTML = presets.map(p => {
    const added = pendingTabs.some(t => t.url === p.url);
    const favicon = faviconUrl(p.url);
    return `<div class="item">
      <img class="item-icon" src="${favicon}" onerror="this.style.display='none';this.nextElementSibling.style.display='block'" />
      <span class="dot" style="background:${p.color};display:none"></span>
      <span class="name">${p.name}</span><span class="url">${p.url}</span>
      ${added ? '<button class="btn-add" disabled>추가됨</button>'
        : `<button class="btn-add" data-name="${p.name}" data-url="${p.url}" data-color="${p.color}">추가</button>`}
    </div>`;
  }).join("");
}

function renderTabList() {
  const list = pendingTabs || tabs;
  document.getElementById("tab-list").innerHTML = list.map((t, i) => {
    const favicon = faviconUrl(t.url);
    return `<div class="item">
    <span class="order-num">${i + 1}</span>
    <button class="btn-arrow" ${i === 0 ? "disabled" : ""} data-from="${i}" data-to="${i - 1}">▲</button>
    <button class="btn-arrow" ${i === list.length - 1 ? "disabled" : ""} data-from="${i}" data-to="${i + 1}">▼</button>
    <img class="item-icon" src="${favicon}" onerror="this.style.display='none';this.nextElementSibling.style.display='block'" />
    <span class="dot" style="background:${t.color};display:none"></span>
    <span class="name">${t.name}</span><span class="url">${t.url}</span>
    ${list.length > 1 ? `<button class="btn-del" data-idx="${i}">삭제</button>` : ""}
  </div>`;
  }).join("");

  document.getElementById("tab-list").querySelectorAll(".btn-arrow:not([disabled])").forEach(btn => {
    btn.addEventListener("click", () => {
      const from = parseInt(btn.dataset.from);
      const to = parseInt(btn.dataset.to);
      const moved = pendingTabs.splice(from, 1)[0];
      pendingTabs.splice(to, 0, moved);
      markDirty();
      renderTabList();
    });
  });

  document.getElementById("tab-list").querySelectorAll(".btn-del").forEach(btn => {
    btn.addEventListener("click", async () => {
      const idx = parseInt(btn.dataset.idx);
      pendingTabs.splice(idx, 1);
      markDirty();
      const presets = await invoke("get_presets");
      renderPresetList(presets);
      renderTabList();
    });
  });
}

async function getVersion() { return await invoke("get_version"); }

async function checkUpdate() {
  try {
    const result = await invoke("check_update");
    if (result) {
      const [version, htmlUrl, assetUrl] = result;
      showUpdateModal(version, htmlUrl, assetUrl);
    }
  } catch (e) {}
}

async function showUpdateModal(version, htmlUrl, assetUrl) {
  // 메인 웹뷰를 전체 크기로 확장 (48px → 전체)
  await invoke("toggle_settings_view", { open: true });
  const currentVer = await getVersion();

  const overlay = document.createElement("div");
  overlay.id = "update-modal";
  overlay.innerHTML = `
    <div class="update-box">
      <h3>새 버전이 있습니다!</h3>
      <p>현재: <b>v${currentVer}</b></p>
      <p>최신: <b>v${version}</b></p>
      <div class="update-btns">
        <button id="update-install">즉시 업그레이드</button>
        <button id="update-page">다운로드 페이지</button>
        <button id="update-no">닫기</button>
      </div>
      <p id="update-status" style="display:none;margin-top:12px;font-size:12px;color:#a6e3a1"></p>
    </div>`;
  document.body.appendChild(overlay);

  // 모달 닫기 공통 처리: 웹뷰 복원
  async function closeModal() {
    overlay.remove();
    await invoke("toggle_settings_view", { open: false });
  }

  // 즉시 업그레이드: 다운로드 후 설치
  document.getElementById("update-install").addEventListener("click", async () => {
    const btn = document.getElementById("update-install");
    const status = document.getElementById("update-status");
    btn.disabled = true;
    btn.textContent = "다운로드 중...";
    status.style.display = "block";
    status.textContent = "설치파일을 다운로드하고 있습니다...";
    try {
      await invoke("download_and_install", { downloadUrl: assetUrl });
    } catch (e) {
      status.style.color = "#f38ba8";
      status.textContent = "실패: " + e;
      btn.disabled = false;
      btn.textContent = "즉시 업그레이드";
    }
  });

  // 다운로드 페이지 열기
  document.getElementById("update-page").addEventListener("click", async () => {
    window.__TAURI__.opener.openUrl(htmlUrl);
    await closeModal();
  });

  document.getElementById("update-no").addEventListener("click", () => closeModal());
}
