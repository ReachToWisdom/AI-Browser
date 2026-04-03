package main

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// 설정 페이지 HTML
func generateSettingsHTML() string {
	tabMutex.Lock()
	tabs := make([]TabItem, len(allTabs))
	copy(tabs, allTabs)
	tabMutex.Unlock()

	tabsJSON, _ := json.Marshal(tabs)
	presetsJSON, _ := json.Marshal(presetTabs)

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>설정</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',sans-serif;background:#1e1e2e;color:#cdd6f4;padding:30px 40px}
h2{font-size:18px;margin:20px 0 12px;color:#cba6f7}
h2:first-child{margin-top:0}
.item{display:flex;align-items:center;gap:10px;padding:8px 12px;
background:#313244;border-radius:8px;margin-bottom:6px}
.dot{width:10px;height:10px;border-radius:50%%;flex-shrink:0}
.name{font-weight:500;min-width:80px}
.url{color:#6c7086;font-size:12px;flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.btn-del{background:#f38ba8;color:#1e1e2e;border:none;border-radius:4px;
padding:3px 10px;cursor:pointer;font-size:12px;font-weight:600}
.btn-del:hover{background:#eba0ac}
.btn-add{background:#a6e3a1;color:#1e1e2e;border:none;border-radius:4px;
padding:3px 10px;cursor:pointer;font-size:12px;font-weight:600}
.btn-add:hover{background:#94e2d5}
.btn-add:disabled{background:#45475a;color:#6c7086;cursor:default}
.form{display:flex;gap:8px;margin:10px 0}
.form input{background:#313244;border:1px solid #45475a;color:#cdd6f4;
border-radius:6px;padding:6px 10px;font-size:13px}
.form input:first-child{width:100px}
.form input:nth-child(2){flex:1}
.back{background:#89b4fa;color:#1e1e2e;border:none;border-radius:6px;
padding:8px 20px;cursor:pointer;font-weight:600;font-size:14px;margin-top:16px}
.back:hover{background:#74c7ec}
.hint{color:#585b70;font-size:11px;margin-top:6px}
.order-num{color:#6c7086;font-size:13px;min-width:20px;text-align:center}
.btn-arrow{background:none;border:1px solid #45475a;color:#cdd6f4;border-radius:4px;
width:24px;height:24px;cursor:pointer;font-size:14px;line-height:1;padding:0}
.btn-arrow:hover{background:#45475a}
.btn-arrow:disabled{opacity:0.2;cursor:default;background:none}
</style></head><body>
<h2>📌 AI 프리셋 (원클릭 추가)</h2>
<div id="presets"></div>

<h2>📋 현재 탭</h2>
<div id="tabs"></div>
<p class="hint">▲▼ 화살표로 순서 변경. 최소 1개 유지.</p>

<h2>➕ 직접 추가</h2>
<div class="form">
<input id="n" placeholder="이름">
<input id="u" placeholder="https://...">
<button class="btn-add" onclick="doAdd()">추가</button>
</div>

<button class="back" onclick="goBackToTab()">← 돌아가기</button>

<div id="about" style="margin-top:32px;padding-top:16px;border-top:1px solid #45475a;color:#a6adc8;font-size:13px;text-align:center"></div>

<script>
var tabs=%s;
var presets=%s;

function bgrToHex(c){
  var r=c&0xFF,g=(c>>8)&0xFF,b=(c>>16)&0xFF;
  return '#'+[r,g,b].map(function(v){return v.toString(16).padStart(2,'0')}).join('');
}
function hasTab(url){
  return tabs.some(function(t){return t.url===url});
}
function renderPresets(){
  var h='';
  presets.forEach(function(p){
    var added=hasTab(p.url);
    h+='<div class="item"><div class="dot" style="background:'+bgrToHex(p.color)+'"></div>'
      +'<span class="name">'+p.name+'</span>'
      +'<span class="url">'+p.url+'</span>'
      +(added?'<button class="btn-add" disabled>추가됨</button>'
             :'<button class="btn-add" onclick="addPreset(\''+p.name+'\',\''+p.url+'\','+p.color+')">추가</button>')
      +'</div>';
  });
  document.getElementById('presets').innerHTML=h;
}
function moveTab(from,to){
  if(to<0||to>=tabs.length)return;
  reorderTabs(from,to).then(function(){
    var t=tabs.splice(from,1)[0];tabs.splice(to,0,t);
    renderPresets();renderTabs();
  });
}
function renderTabs(){
  var h='';
  tabs.forEach(function(t,i){
    h+='<div class="item">'
      +'<span class="order-num">'+(i+1)+'</span>'
      +'<button class="btn-arrow" onclick="moveTab('+i+','+(i-1)+')"'+(i===0?' disabled':'')+'>▲</button>'
      +'<button class="btn-arrow" onclick="moveTab('+i+','+(i+1)+')"'+(i===tabs.length-1?' disabled':'')+'>▼</button>'
      +'<div class="dot" style="background:'+bgrToHex(t.color)+'"></div>'
      +'<span class="name">'+t.name+'</span>'
      +'<span class="url">'+t.url+'</span>'
      +(tabs.length>1?'<button class="btn-del" onclick="doRemove('+i+')">삭제</button>':'')
      +'</div>';
  });
  document.getElementById('tabs').innerHTML=h;
}
function addPreset(n,u,c){
  addNewTab(n,u,c).then(function(){
    tabs.push({name:n,url:u,color:c});
    renderPresets();renderTabs();
  });
}
function doRemove(i){
  removeTab(i).then(function(){
    tabs.splice(i,1);renderPresets();renderTabs();
  });
}
function doAdd(){
  var n=document.getElementById('n').value.trim();
  var u=document.getElementById('u').value.trim();
  if(!n||!u){alert('이름과 URL을 입력하세요');return;}
  addNewTab(n,u,0x888888).then(function(){
    tabs.push({name:n,url:u,color:0x888888});
    renderPresets();renderTabs();
    document.getElementById('n').value='';
    document.getElementById('u').value='';
  });
}
renderPresets();renderTabs();
document.getElementById('about').textContent='%s v%s \u00B7 개발자: %s';
</script></body></html>`, string(tabsJSON), string(presetsJSON), APP_NAME, APP_VERSION, APP_DEVELOPER)
}

func openSettings() {
	if webviewInstance == nil {
		return
	}
	webviewInstance.Dispatch(func() {
		openSettingsView()
		html := generateSettingsHTML()
		escaped := url.PathEscape(html)
		webviewInstance.Navigate("data:text/html;charset=utf-8," + escaped)
	})
}
