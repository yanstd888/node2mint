// Bukti E2E: konfirmasi destruktif memakai Dialog bersama, bukan window.confirm native.
// Jalankan dari web/: node check-confirm.mjs   (butuh next dev di :5000, gateway :8080, akun admin dev)
import { spawn } from 'node:child_process';
import { setTimeout as sleep } from 'node:timers/promises';

const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const PORT = 9336;
// Cookie sesi gateway bersifat host-only untuk "localhost" (cookie mengabaikan port),
// jadi navigasi WAJIB lewat http://localhost:5000, bukan 127.0.0.1.
const BASE = 'http://localhost:5000';
const EMAIL = 'admin@test.local';
const PASSWORD = 'Test1234!';   // akun dev yang sudah tercatat di repo (ARGUS-PENTEST-BRIEF.md)

const chrome = spawn(CHROME, [
  '--headless=new', '--disable-gpu', `--remote-debugging-port=${PORT}`,
  '--user-data-dir=' + process.env.TEMP + '/cdp-confirm', '--window-size=1280,900', 'about:blank',
], { stdio: 'ignore' });

for (let i = 0; i < 40; i++) {
  try { await fetch(`http://127.0.0.1:${PORT}/json/version`); break; } catch { await sleep(250); }
}
const tab = await (await fetch(`http://127.0.0.1:${PORT}/json/new?about:blank`, { method: 'PUT' })).json();
const ws = new WebSocket(tab.webSocketDebuggerUrl);
await new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej; });

let id = 0;
const tunggu = new Map();
let SISIP_COOKIE = null;
// Chrome MENGABAIKAN header Cookie di Network.setExtraHTTPHeaders, dan cookie jar CDP tidak
// dipakai untuk permintaan halaman di headless. Satu-satunya cara andal: intersepsi Fetch.
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.method === 'Fetch.requestPaused') {
    const r = m.params.request;
    const headers = r.headers ? Object.entries(r.headers).map(([name, value]) => ({ name, value })) : [];
    if (SISIP_COOKIE && r.url.includes('localhost')) headers.push({ name: 'Cookie', value: SISIP_COOKIE });
    ws.send(JSON.stringify({ id: ++id, method: 'Fetch.continueRequest', params: { requestId: m.params.requestId, headers } }));
    return;
  }
  if (m.id && tunggu.has(m.id)) { tunggu.get(m.id)(m); tunggu.delete(m.id); }
};
const cmd = (method, params = {}) => new Promise((res) => {
  const n = ++id; tunggu.set(n, res);
  ws.send(JSON.stringify({ id: n, method, params }));
});
const evalJs = async (expression) => {
  const r = await cmd('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
  if (r.error) throw new Error(JSON.stringify(r.error));
  if (r.result.exceptionDetails) throw new Error(r.result.exceptionDetails.text);
  return r.result.result.value;
};
const goto = async (url) => { await cmd('Page.navigate', { url }); await sleep(1200); };
const tungguSampai = async (expr, label, batas = 40) => {
  for (let i = 0; i < batas; i++) {
    if (await evalJs(`!!(${expr})`)) return true;
    await sleep(250);
  }
  throw new Error('timeout menunggu: ' + label);
};

await cmd('Runtime.enable');

// --- 1. login admin lewat gateway, ambil cookie sesi HttpOnly ---
const login = await fetch('http://127.0.0.1:8080/api/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: EMAIL, password: PASSWORD }),
});
if (!login.ok) throw new Error('login gateway gagal: HTTP ' + login.status);
SISIP_COOKIE = login.headers.getSetCookie()[0].split(';')[0];
if (!SISIP_COOKIE) throw new Error('tidak ada cookie sesi dari gateway');
console.log('login ok (cookie sesi didapat)');

// --- 2. halaman paket admin; pasang jebakan untuk confirm() native ---
await cmd('Fetch.enable', { patterns: [{ urlPattern: '*', requestStage: 'Request' }] });
await goto(`${BASE}/admin/packages`);
await tungguSampai(`document.querySelector('button[aria-label^="Hapus"], button[aria-label^="Delete"]')`, 'tabel paket');

// Kalau kode masih memanggil window.confirm, jebakan ini berbunyi dan mengembalikan true
// (artinya baris ikut terhapus) — kita hanya ingin membuktikan ia TIDAK dipanggil.
await evalJs(`(() => {
  window.__nativeConfirmDipanggil = false;
  window.confirm = () => { window.__nativeConfirmDipanggil = true; return false; };
  return true;
})()`);

const sebelum = await evalJs(`document.querySelectorAll('button[aria-label^="Hapus"], button[aria-label^="Delete"]').length`);
await evalJs(`document.querySelector('button[aria-label^="Hapus"], button[aria-label^="Delete"]').click()`);
const dialogMuncul = await evalJs(`!!document.querySelector('[role="dialog"]')`);
const tombolDialog = await evalJs(`JSON.stringify([...document.querySelectorAll('[role="dialog"] button')].map((b) => b.textContent.trim()))`);
const native = await evalJs(`window.__nativeConfirmDipanggil`);

// tombol pertama = Batal (fokus awal jatuh ke sisi aman)
await evalJs(`document.querySelectorAll('[role="dialog"] button')[0].click()`);
await sleep(400);
const dialogTutup = !(await evalJs(`!!document.querySelector('[role="dialog"]')`));
const sesudah = await evalJs(`document.querySelectorAll('button[aria-label^="Hapus"], button[aria-label^="Delete"]').length`);

console.log('baris sebelum        =', sebelum);
console.log('dialog muncul        =', dialogMuncul);
console.log('tombol dialog        =', tombolDialog);
console.log('confirm() native?    =', native, native ? '<-- MASIH DIPANGGIL' : '(tidak dipanggil)');
console.log('dialog tertutup Batal=', dialogTutup);
console.log('baris sesudah Batal  =', sesudah, sesudah === sebelum ? '(tidak ada yang terhapus)' : '<-- BARIS HILANG');

const lulus = dialogMuncul && !native && dialogTutup && sesudah === sebelum && sebelum > 0;
console.log(lulus ? 'LULUS' : 'GAGAL');
ws.close(); chrome.kill();
process.exit(lulus ? 0 : 1);
