const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await context.newPage();

  context.on('page', async (newPage) => {
    console.log(`[새 창 열림] URL: ${newPage.url()}`);
    newPage.on('load', () => console.log(`[새 창 로드] URL: ${newPage.url()}`));
  });

  await page.goto('https://www.naver.com');
  console.log('네이버 접속 완료. 로그인 후 60초 대기...');
  await page.waitForTimeout(60000);

  // 카페 관련 링크 분석
  console.log('\n=== 카페 관련 링크/요소 분석 ===');
  const cafeElements = await page.evaluate(() => {
    const results = [];
    // 모든 링크
    document.querySelectorAll('a').forEach(a => {
      const text = a.textContent.trim();
      const href = a.href;
      if (text.includes('카페') || href.includes('cafe')) {
        results.push({
          type: 'a',
          text: text.substring(0, 80),
          href: href,
          target: a.target || '(없음)',
          onclick: a.getAttribute('onclick') || '(없음)',
          className: a.className.substring(0, 80),
          parentClass: a.parentElement?.className?.substring(0, 50) || ''
        });
      }
    });
    // 버튼 등
    document.querySelectorAll('button, [role="button"]').forEach(el => {
      const text = el.textContent.trim();
      if (text.includes('카페')) {
        results.push({
          type: el.tagName,
          text: text.substring(0, 80),
          href: '(없음)',
          target: '(없음)',
          onclick: el.getAttribute('onclick') || '(없음)',
          className: el.className.substring(0, 80),
          parentClass: el.parentElement?.className?.substring(0, 50) || ''
        });
      }
    });
    return results;
  });

  console.log(`카페 관련 요소 ${cafeElements.length}개 발견:`);
  cafeElements.forEach((el, i) => {
    console.log(`\n[${i+1}] <${el.type}> "${el.text}"`);
    console.log(`  href: ${el.href}`);
    console.log(`  target: ${el.target}`);
    console.log(`  onclick: ${el.onclick}`);
    console.log(`  class: ${el.className}`);
    console.log(`  parent: ${el.parentClass}`);
  });

  // MyView 카페 패널 내부 링크 분석
  console.log('\n=== MyView 카페 패널 내부 분석 ===');
  const myviewCafe = await page.evaluate(() => {
    const results = [];
    // MyView 영역 내 카페 관련 요소
    const myview = document.querySelector('[class*="MyView"]') || document.querySelector('.MyView');
    if (myview) {
      myview.querySelectorAll('a, button, [role="button"], [onclick]').forEach(el => {
        const text = el.textContent.trim();
        if (text.includes('카페') || el.href?.includes('cafe')) {
          results.push({
            tag: el.tagName,
            text: text.substring(0, 80),
            href: el.href || '(없음)',
            target: el.target || '(없음)',
            onclick: el.getAttribute('onclick') || '(없음)',
            className: el.className.substring(0, 80),
            role: el.getAttribute('role') || '(없음)'
          });
        }
      });
    }
    return { found: !!myview, elements: results };
  });
  console.log(`MyView 발견: ${myviewCafe.found}, 요소: ${myviewCafe.elements.length}개`);
  myviewCafe.elements.forEach((el, i) => {
    console.log(`[${i+1}] <${el.tag}> "${el.text}" href=${el.href} target=${el.target} onclick=${el.onclick}`);
  });

  console.log('\n카페 바로가기를 클릭해주세요. 60초 대기...');

  // window.open 감시
  await page.evaluate(() => {
    const orig = window.open;
    window.open = function(...args) {
      console.log('[window.open]', JSON.stringify(args));
      return orig.apply(this, args);
    };
  });
  page.on('console', msg => console.log(`[콘솔] ${msg.text()}`));
  page.on('popup', p => console.log(`[팝업] ${p.url()}`));

  await page.waitForTimeout(60000);
  console.log('테스트 종료');
  await browser.close();
})();
