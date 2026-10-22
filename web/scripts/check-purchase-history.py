"""Production regression: completed token-package buys appear in dashboard and order history."""
import os
import time
from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By

email = os.environ['AISELL_TEST_EMAIL']
password = os.environ['AISELL_TEST_PASSWORD']
opts = Options()
opts.add_argument('--headless=new')
opts.add_argument('--no-first-run')
opts.add_argument(f"--user-data-dir={os.environ['TMPDIR']}/aisell-purchase-history")
driver = webdriver.Chrome(options=opts)

try:
    driver.get('http://127.0.0.1:5000/masuk')
    time.sleep(1)
    driver.find_element(By.ID, 'email').send_keys(email)
    driver.find_element(By.ID, 'password').send_keys(password)
    driver.find_element(By.ID, 'password').submit()
    for _ in range(20):
        if '/dashboard' in driver.current_url:
            break
        time.sleep(.25)
    assert '/dashboard' in driver.current_url, driver.current_url

    for _ in range(20):
        if 'Beli paket ' in driver.find_element(By.TAG_NAME, 'body').text:
            break
        time.sleep(.25)
    assert 'Beli paket ' in driver.find_element(By.TAG_NAME, 'body').text

    driver.get('http://127.0.0.1:5000/pesanan')
    rows = []
    for _ in range(20):
        rows = driver.find_elements(By.XPATH, "//li[contains(., 'Beli paket ')]")
        if rows:
            break
        time.sleep(.25)
    assert rows, 'package purchase missing from /pesanan'
    assert not rows[0].find_elements(By.TAG_NAME, 'button'), 'package history must not expose marketplace actions'
    print('PASS completed token-package purchase appears in /dashboard and /pesanan')
finally:
    driver.quit()
