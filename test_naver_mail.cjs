const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await context.newPage();

  // 새 창/팝업 이벤트 감지
  context.on('page', async (newPage) => {
    const url = newPage.url();
    console.log(`[새 창 열림] URL: ${url}`);
    newPage.on('load', () => {
      console.log(`[새 창 로드 완료] URL: ${newPage.url()}`);
    });
  });

  // 네이버 접속
  await page.goto('https://www.naver.com');
  console.log('네이버 접속 완료. 로그인해주세요.');
  console.log('로그인 후 Enter를 누르세요...');

  // 사용자 로그인 대기 (60초)
  await page.waitForTimeout(60000);

  // 메일 관련 링크 분석
  console.log('\n=== 메일 관련 링크 분석 ===');
  const mailLinks = await page.evaluate(() => {
    const results = [];
    const allLinks = document.querySelectorAll('a');
    allLinks.forEach(a => {
      const text = a.textContent.trim();
      const href = a.href;
      const target = a.target;
      const onclick = a.getAttribute('onclick');
      if (text.includes('메일') || href.includes('mail')) {
        results.push({
          text: text.substring(0, 50),
          href: href,
          target: target || '(없음)',
          onclick: onclick || '(없음)',
          tagName: a.tagName,
          className: a.className.substring(0, 50)
        });
      }
    });
    return results;
  });

  console.log(`메일 관련 링크 ${mailLinks.length}개 발견:`);
  mailLinks.forEach((link, i) => {
    console.log(`\n[${i+1}] "${link.text}"`);
    console.log(`  href: ${link.href}`);
    console.log(`  target: ${link.target}`);
    console.log(`  onclick: ${link.onclick}`);
    console.log(`  class: ${link.className}`);
  });

  // window.open 호출 감지
  await page.evaluate(() => {
    const origOpen = window.open;
    window.open = function(...args) {
      console.log('[window.open 호출]', JSON.stringify(args));
      return origOpen.apply(this, args);
    };
  });

  console.log('\n이제 메일 아이콘을 클릭해주세요. 60초간 대기합니다...');

  // 콘솔 로그 캡처
  page.on('console', msg => {
    if (msg.text().includes('window.open')) {
      console.log(`[페이지 콘솔] ${msg.text()}`);
    }
  });

  // 팝업 이벤트 대기
  page.on('popup', async popup => {
    console.log(`[팝업 감지] URL: ${popup.url()}`);
  });

  await page.waitForTimeout(60000);

  console.log('\n테스트 종료');
  await browser.close();
})();
