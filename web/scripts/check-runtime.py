"""Smoke test for production hydration; run while `npm run start` serves port 5000."""
import json
import os
import time
from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By

opts = Options()
opts.add_argument('--headless=new')
opts.add_argument('--no-first-run')
opts.add_argument(f"--user-data-dir={os.environ['TMPDIR']}/aisell-runtime-check")
opts.add_argument('--window-size=1440,1000')
opts.set_capability('goog:loggingPrefs', {'browser': 'ALL'})
driver = webdriver.Chrome(options=opts)

try:
    driver.get('http://127.0.0.1:5000/')
    time.sleep(1)
    driver.find_element(By.XPATH, "//button[translate(normalize-space(.),'ID','id')='id']").click()
    time.sleep(.2)
    assert driver.find_element(By.TAG_NAME, 'html').get_attribute('lang') == 'id'
    assert 'Bangun dengan AI.' in driver.find_element(By.TAG_NAME, 'body').text

    driver.find_element(By.XPATH, "//button[translate(normalize-space(.),'EN','en')='en']").click()
    time.sleep(.2)
    assert driver.find_element(By.TAG_NAME, 'html').get_attribute('lang') == 'en'
    assert 'Build with AI.' in driver.find_element(By.TAG_NAME, 'body').text

    driver.get('http://127.0.0.1:5000/masuk?aff=REFTEST')
    time.sleep(1)
    assert driver.find_element(By.ID, 'full-name')
    assert driver.find_element(By.ID, 'email')
    password = driver.find_element(By.ID, 'password')
    password.send_keys('short')
    assert 'Password must contain 8–72 characters.' in driver.find_element(By.TAG_NAME, 'body').text
    driver.find_element(By.CSS_SELECTOR, 'button[aria-label="Show password"]').click()
    assert driver.find_element(By.ID, 'password').get_attribute('type') == 'text'
    confirm = driver.find_element(By.ID, 'password-confirm')
    confirm.send_keys('different-password')
    assert 'Password confirmation does not match.' in driver.find_element(By.TAG_NAME, 'body').text
    assert driver.find_element(By.ID, 'referral-code').get_attribute('value') == 'REFTEST'

    driver.get('http://127.0.0.1:5000/dashboard')
    time.sleep(1)
    assert driver.current_url.endswith('/masuk')
    assert not driver.find_elements(By.CSS_SELECTOR, '.skel')

    severe = [x['message'] for x in driver.get_log('browser') if x['level'] == 'SEVERE' and '/api/dashboard' not in x['message']]
    assert not severe, severe
    print(json.dumps({'result': 'PASS', 'unexpected_console_errors': severe}))
finally:
    driver.quit()
