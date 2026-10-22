"""Verify locale switching never stacks URL prefixes or mounts the app shell."""
import os
import time
from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By

base = os.getenv('BASE_URL', 'http://127.0.0.1:5000').rstrip('/')
opts = Options()
opts.add_argument('--headless=new')
opts.add_argument('--no-first-run')
opts.add_argument(f"--user-data-dir={os.environ['TMPDIR']}/node2mint-locale-check")
opts.add_argument('--window-size=1440,1000')
driver = webdriver.Chrome(options=opts)

try:
    driver.get(f'{base}/en')
    for _ in range(40):
        if driver.find_elements(By.CSS_SELECTOR, 'header summary'):
            break
        time.sleep(.1)
    for button, expected in [('id', '/id'), ('cn', '/zh'), ('ru', '/ru'), ('jp', '/ja')]:
        driver.find_element(By.CSS_SELECTOR, 'header summary').click()
        driver.find_element(By.XPATH, f"//button[translate(normalize-space(.),'ABCDEFGHIJKLMNOPQRSTUVWXYZ','abcdefghijklmnopqrstuvwxyz')='{button}']").click()
        for _ in range(40):
            if driver.current_url.split('?', 1)[0].endswith(expected):
                break
            time.sleep(.1)
        assert driver.current_url.split('?', 1)[0].endswith(expected), driver.current_url
        for _ in range(40):
            if not driver.find_elements(By.CSS_SELECTOR, 'aside.fixed.left-0'):
                break
            time.sleep(.1)
        assert not driver.find_elements(By.CSS_SELECTOR, 'aside.fixed.left-0'), f'app shell visible at {driver.current_url}'
    print('PASS locale URLs are singular and public shell stays hidden')
finally:
    driver.quit()
