"""Regression: choosing Wallet opens confirmation and must not call a purchase endpoint before consent."""
import os
import time
from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By

opts = Options()
opts.add_argument('--headless=new')
opts.add_argument('--no-first-run')
opts.add_argument(f"--user-data-dir={os.environ['TMPDIR']}/aisell-payment-confirm")
driver = webdriver.Chrome(options=opts)

driver.execute_cdp_cmd('Page.addScriptToEvaluateOnNewDocument', {'source': '''
(() => {
  const OriginalXHR = window.XMLHttpRequest;
  window.__purchaseRequests = [];
  window.XMLHttpRequest = function() {
    const xhr = new OriginalXHR();
    let url = '';
    const open = xhr.open.bind(xhr);
    const send = xhr.send.bind(xhr);
    xhr.open = function(method, nextUrl) { url = String(nextUrl); return open.apply(xhr, arguments); };
    const respond = (body, delay = 0) => {
      Object.defineProperties(xhr, {
        readyState: { get: () => 4 }, status: { get: () => 200 }, statusText: { get: () => 'OK' },
        responseText: { get: () => body }, response: { get: () => body },
      });
      return setTimeout(() => { xhr.dispatchEvent(new Event('readystatechange')); xhr.dispatchEvent(new Event('load')); xhr.dispatchEvent(new Event('loadend')); }, delay);
    };
    xhr.send = function() {
      if (url.endsWith('/api/dashboard')) return respond('{"user_id":"payment-test","email":"payment-test@example.invalid","role":"USER","balance":100000}');
      if (/\\/api\\/(packages\\/[^/]+\\/buy|market\\/orders|market\\/checkout)$/.test(url)) {
        window.__purchaseRequests.push(url);
        return respond('{"id":"payment-test-order"}', 250);
      }
      return send.apply(xhr, arguments);
    };
    return xhr;
  };
})();
'''})

try:
    driver.get('http://127.0.0.1:5000/payment?kind=package&amount=4000&label=5.2M+%2F+24h&id=ee7254e5-8bb5-4038-9fa1-85a5fca40f8e')
    time.sleep(1)
    driver.find_element(By.XPATH, "//button[.//span[normalize-space()='Pay with Wallet']]").click()
    dialog = driver.find_element(By.CSS_SELECTOR, '[role="dialog"]')
    assert 'Confirm wallet payment' in dialog.text
    assert '5.2M / 24h' in dialog.text
    assert driver.execute_script('return window.__purchaseRequests') == []
    dialog.find_element(By.XPATH, ".//button[normalize-space()='Cancel']").click()
    time.sleep(.2)
    assert driver.execute_script('return window.__purchaseRequests') == []

    driver.find_element(By.XPATH, "//button[.//span[normalize-space()='Pay with Wallet']]").click()
    driver.find_element(By.XPATH, "//div[@role='dialog']//button[normalize-space()='Confirm purchase']").click()
    time.sleep(.05)
    assert 'Processing…' in driver.find_element(By.TAG_NAME, 'body').text
    assert len(driver.execute_script('return window.__purchaseRequests')) == 1
    time.sleep(.4)
    receipt_body = driver.find_element(By.TAG_NAME, 'body').text
    assert 'PAYMENT SUCCESSFUL' in receipt_body.upper(), receipt_body
    print('PASS wallet confirmation blocks purchase request until consent, then processes before receipt')
finally:
    driver.quit()
