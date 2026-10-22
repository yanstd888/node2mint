"""Regression: compact and expanded sidebars must never show native scroll rails."""

from __future__ import annotations

import os
import time
from pathlib import Path

from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait

BASE_URL = os.getenv("BASE_URL", "http://127.0.0.1:5000").rstrip("/")
OUTPUT_DIR = Path(os.environ["TMPDIR"]) / "node2mint-sidebar-layout"

MOCK_API = r'''
(() => {
  try { localStorage.setItem('ui.sidebar.collapsed', '0'); } catch {}
  const OriginalXHR = window.XMLHttpRequest;
  window.XMLHttpRequest = function() {
    const xhr = new OriginalXHR();
    let url = '';
    const open = xhr.open.bind(xhr);
    const send = xhr.send.bind(xhr);
    xhr.open = function(method, nextUrl) {
      url = String(nextUrl);
      return open.apply(xhr, arguments);
    };
    const respond = (body) => {
      Object.defineProperties(xhr, {
        readyState: { get: () => 4 },
        status: { get: () => 200 },
        statusText: { get: () => 'OK' },
        responseText: { get: () => body },
        response: { get: () => body },
      });
      setTimeout(() => {
        xhr.dispatchEvent(new Event('readystatechange'));
        xhr.dispatchEvent(new Event('load'));
        xhr.dispatchEvent(new Event('loadend'));
      });
    };
    xhr.send = function() {
      if (url.endsWith('/api/dashboard')) {
        return respond('{"user_id":"sidebar-test","email":"sidebar-test@example.invalid","name":"Yan Std","role":"USER","balance":50000000,"tokenRemaining":1000000,"tokenActive":true}');
      }
      if (url.endsWith('/api/notifications/unread')) return respond('{"count":0,"notifications":[]}');
      if (url.endsWith('/api/cart')) return respond('{"items":[]}');
      if (url.endsWith('/api/store')) return respond('{"store":null}');
      if (url.endsWith('/api/purchases')) return respond('{"orders":[]}');
      if (url.endsWith('/api/keys/usage')) return respond('{"totals":{"requests":0,"input_tokens":0,"cached_tokens":0,"output_tokens":0},"recent":[]}');
      if (url.endsWith('/api/affiliate/links')) return respond('{"summary":{"clicks":0,"conversions":0,"earnings":0}}');
      return send.apply(xhr, arguments);
    };
    return xhr;
  };
})();
'''


def metrics(driver):
    return driver.execute_script(
        """
        const aside = document.querySelector('aside');
        const nav = aside?.querySelector('nav.sidebar-nav');
        const desktopHeader = [...document.querySelectorAll('header')]
          .find((header) => String(header.className).includes('md:flex'));
        return {
          sidebarWidth: Math.round(aside?.getBoundingClientRect().width || 0),
          scrollable: Boolean(nav && nav.scrollHeight > nav.clientHeight),
          horizontalOverflow: Boolean(nav && nav.scrollWidth > nav.clientWidth),
          nativeRailWidth: nav ? Math.max(0, nav.offsetWidth - nav.clientWidth) : -1,
          scrollbarWidth: nav ? getComputedStyle(nav).scrollbarWidth : '',
          headerBrandLogoCount: desktopHeader?.querySelectorAll('img[src*="node2mint-icon"]').length ?? -1,
        };
        """
    )


def assert_layout(values, expected_width, expected_header_logos):
    assert values["sidebarWidth"] == expected_width, values
    assert values["scrollable"], values
    assert not values["horizontalOverflow"], values
    assert values["nativeRailWidth"] <= 1, values
    assert values["scrollbarWidth"] == "none", values
    assert values["headerBrandLogoCount"] == expected_header_logos, values


opts = Options()
opts.add_argument("--headless=new")
opts.add_argument("--no-first-run")
opts.add_argument("--window-size=1440,900")
opts.add_argument(f"--user-data-dir={os.environ['TMPDIR']}/aisell-sidebar-layout")
driver = webdriver.Chrome(options=opts)

try:
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    driver.execute_cdp_cmd("Page.addScriptToEvaluateOnNewDocument", {"source": MOCK_API})
    driver.get(f"{BASE_URL}/id/dashboard?sidebar-layout-check=1")
    WebDriverWait(driver, 12).until(lambda current: current.find_elements(By.CSS_SELECTOR, "aside nav.sidebar-nav"))
    WebDriverWait(driver, 12).until(lambda current: current.execute_script("return document.querySelector('aside nav.sidebar-nav')?.scrollHeight > document.querySelector('aside nav.sidebar-nav')?.clientHeight"))
    time.sleep(0.3)

    expanded = metrics(driver)
    assert_layout(expanded, 256, 0)
    expanded_path = OUTPUT_DIR / "sidebar-expanded.png"
    driver.save_screenshot(str(expanded_path))

    driver.find_element(By.CSS_SELECTOR, 'header button[aria-pressed="false"]').click()
    WebDriverWait(driver, 4).until(lambda current: metrics(current)["sidebarWidth"] == 64)
    time.sleep(0.3)

    compact = metrics(driver)
    assert_layout(compact, 64, 0)
    compact_path = OUTPUT_DIR / "sidebar-compact.png"
    driver.save_screenshot(str(compact_path))

    print({"result": "PASS", "expanded": expanded, "compact": compact, "screenshots": [str(expanded_path), str(compact_path)]})
finally:
    driver.quit()
