const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await context.newPage();
  await page.goto('https://www.naver.com');
  console.log('로그인 후 카페 패널을 열어주세요. 60초 대기...');
  await page.waitForTimeout(60000);

  // iframe 분석
  console.log('\n=== iframe 분석 ===');
  const iframes = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('iframe')).map(f => ({
      src: f.src,
      id: f.id,
      name: f.name,
      className: f.className.substring(0, 50)
    }));
  });
  console.log(`iframe ${iframes.length}개:`);
  iframes.forEach((f, i) => console.log(`[${i+1}] id=${f.id} name=${f.name} src=${f.src} class=${f.className}`));

  // 카페 패널 내 클릭 가능한 요소 (MyView 영역)
  console.log('\n=== 카페 패널 클릭 가능 요소 ===');
  const cafePanel = await page.evaluate(() => {
    const results = [];
    // MyView 카페 탭이 열린 상태에서의 목록 링크
    document.querySelectorAll('a[href*="cafe.naver.com"]').forEach(a => {
      results.push({
        text: a.textContent.trim().substring(0, 60),
        href: a.href,
        target: a.target || '(없음)',
        onclick: a.getAttribute('onclick') || '(없음)',
      });
    });
    // window.open을 사용하는 이벤트 핸들러가 있는 요소
    document.querySelectorAll('[class*="MyView"] a, [class*="MyView"] button').forEach(el => {
      if (!el.href?.includes('cafe')) return;
      results.push({
        text: el.textContent.trim().substring(0, 60),
        href: el.href || '(없음)',
        target: el.target || '(없음)',
        onclick: el.getAttribute('onclick') || '(없음)',
      });
    });
    return results;
  });
  console.log(`카페 링크 ${cafePanel.length}개:`);
  cafePanel.forEach((el, i) => {
    console.log(`[${i+1}] "${el.text}" href=${el.href} target=${el.target}`);
  });

  // 각 frame 내부 확인
  for (const frame of page.frames()) {
    if (frame === page.mainFrame()) continue;
    const url = frame.url();
    if (url.includes('cafe') || url.includes('myview')) {
      console.log(`\n=== Frame: ${url} ===`);
      const links = await frame.evaluate(() => {
        return Array.from(document.querySelectorAll('a')).slice(0, 10).map(a => ({
          text: a.textContent.trim().substring(0, 60),
          href: a.href,
          target: a.target || '(없음)'
        }));
      }).catch(() => []);
      links.forEach(l => console.log(`  "${l.text}" → ${l.href} target=${l.target}`));
    }
  }

  console.log('\n테스트 종료');
  await browser.close();
})();
