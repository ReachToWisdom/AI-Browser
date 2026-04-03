const { invoke } = window.__TAURI__.core;

let tabs = [];
let activeTab = 0;
let settingsOpen = false;

window.addEventListener("DOMContentLoaded", async () => {
  tabs = await invoke("get_tabs");
  activeTab = await invoke("get_active_tab");
  renderTabBar();
  setTimeout(checkUpdate, 1000);
});

// 탭바 렌더링
function renderTabBar() {
  const bar = document.getElementById("tabbar");
  let html = '<button class="tab-btn" id="btn-back" title="뒤로">◀</button>';
  html += '<button class="tab-btn" id="btn-forward" title="앞으로">▶</button>';
  html += '<button class="tab-btn" id="btn-refresh" title="새로고침">↻</button>';
  tabs.forEach((tab, i) => {
    const domain = new URL(tab.url).hostname;
    const favicon = `https://www.google.com/s2/favicons?domain=${domain}&sz=32`;
    html += `<button class="tab${i === activeTab && !settingsOpen ? ' active' : ''}" data-idx="${i}">
      <img class="tab-icon" src="${favicon}" onerror="this.style.display='none';this.nextElementSibling.style.display='block'" />
      <span class="dot" style="background:${tab.color};display:none"></span>
      <span class="name">${tab.name}</span>
    </button>`;
  });
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
  settingsOpen = !settingsOpen;
  document.getElementById("settings").classList.toggle("hidden", !settingsOpen);
  await invoke("toggle_settings_view", { open: settingsOpen });
  if (settingsOpen) renderSettings();
  renderTabBar();
}

// 설정 페이지
async function renderSettings() {
  const presets = await invoke("get_presets");
  tabs = await invoke("get_tabs");

  let html = '<h2>📌 AI 프리셋 (원클릭 추가)</h2><div id="preset-list"></div>';
  html += '<h2>📋 현재 탭</h2><div id="tab-list"></div>';
  html += '<p style="color:#585b70;font-size:11px;margin-top:6px">▲▼ 화살표로 순서 변경. 최소 1개 유지.</p>';
  html += '<h2>➕ 직접 추가</h2>';
  html += '<div class="form"><input id="add-name" placeholder="이름"><input id="add-url" placeholder="https://..."><button class="btn-add" id="btn-do-add">추가</button></div>';
  html += '<button class="back-btn" id="btn-back">← 돌아가기</button>';
  html += `<div class="about">AI Browser v${await getVersion()} · 개발자: 혜통</div>`;
  document.getElementById("settings").innerHTML = html;

  // 프리셋
  document.getElementById("preset-list").innerHTML = presets.map(p => {
    const added = tabs.some(t => t.url === p.url);
    const domain = new URL(p.url).hostname;
    const favicon = `https://www.google.com/s2/favicons?domain=${domain}&sz=32`;
    return `<div class="item">
      <img class="item-icon" src="${favicon}" onerror="this.style.display='none';this.nextElementSibling.style.display='block'" />
      <span class="dot" style="background:${p.color};display:none"></span>
      <span class="name">${p.name}</span><span class="url">${p.url}</span>
      ${added ? '<button class="btn-add" disabled>추가됨</button>'
        : `<button class="btn-add" data-name="${p.name}" data-url="${p.url}" data-color="${p.color}">추가</button>`}
    </div>`;
  }).join("");

  renderTabList();

  document.getElementById("preset-list").addEventListener("click", async (e) => {
    const btn = e.target.closest(".btn-add:not([disabled])");
    if (!btn) return;
    await invoke("add_tab", { name: btn.dataset.name, url: btn.dataset.url, color: btn.dataset.color });
    tabs = await invoke("get_tabs");
    renderSettings();
    renderTabBar();
  });

  document.getElementById("btn-do-add").addEventListener("click", async () => {
    const name = document.getElementById("add-name").value.trim();
    const url = document.getElementById("add-url").value.trim();
    if (!name || !url) { alert("이름과 URL을 입력하세요"); return; }
    await invoke("add_tab", { name, url, color: "#888888" });
    tabs = await invoke("get_tabs");
    renderSettings();
    renderTabBar();
  });

  document.getElementById("btn-back").addEventListener("click", toggleSettings);
}

function renderTabList() {
  document.getElementById("tab-list").innerHTML = tabs.map((t, i) => {
    const domain = new URL(t.url).hostname;
    const favicon = `https://www.google.com/s2/favicons?domain=${domain}&sz=32`;
    return `<div class="item">
    <span class="order-num">${i + 1}</span>
    <button class="btn-arrow" ${i === 0 ? "disabled" : ""} data-from="${i}" data-to="${i - 1}">▲</button>
    <button class="btn-arrow" ${i === tabs.length - 1 ? "disabled" : ""} data-from="${i}" data-to="${i + 1}">▼</button>
    <img class="item-icon" src="${favicon}" onerror="this.style.display='none';this.nextElementSibling.style.display='block'" />
    <span class="dot" style="background:${t.color};display:none"></span>
    <span class="name">${t.name}</span><span class="url">${t.url}</span>
    ${tabs.length > 1 ? `<button class="btn-del" data-idx="${i}">삭제</button>` : ""}
  </div>`;
  }).join("");

  document.getElementById("tab-list").querySelectorAll(".btn-arrow:not([disabled])").forEach(btn => {
    btn.addEventListener("click", async () => {
      await invoke("reorder_tab", { from: parseInt(btn.dataset.from), to: parseInt(btn.dataset.to) });
      tabs = await invoke("get_tabs");
      renderTabList();
      renderTabBar();
    });
  });

  document.getElementById("tab-list").querySelectorAll(".btn-del").forEach(btn => {
    btn.addEventListener("click", async () => {
      await invoke("remove_tab", { index: parseInt(btn.dataset.idx) });
      tabs = await invoke("get_tabs");
      if (activeTab >= tabs.length) activeTab = tabs.length - 1;
      renderTabList();
      renderTabBar();
    });
  });
}

async function getVersion() { return "2.0.1"; }

async function checkUpdate() {
  try {
    const result = await invoke("check_update");
    if (result) {
      const [version, url] = result;
      if (confirm(`새 버전이 있습니다!\n\n현재: v2.0.1\n최신: v${version}\n\n다운로드 페이지를 열까요?`)) {
        window.__TAURI__.opener.openUrl(url);
      }
    }
  } catch (e) {}
}
