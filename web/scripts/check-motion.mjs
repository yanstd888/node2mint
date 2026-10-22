// Uji nyata stagger + hover-lift lewat CDP (WebSocket native Node, tanpa dependensi).
// Jalankan: node check-motion.mjs   (butuh next dev di :5000)
import { spawn } from 'node:child_process';
import { setTimeout as sleep } from 'node:timers/promises';

const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const PORT = 9333;
const BASE = 'http://localhost:5000';
const HALAMAN = [
  '/', '/pricing', '/faq', '/market', '/market?q=a',   // /market?q=a: halaman produk nyata ber-.lift
  '/docs', '/status',
  // Halaman ber-login (/dashboard, /wallet) sengaja tidak diuji: tanpa sesi keduanya redirect ke /masuk.
];

const chrome = spawn(CHROME, [
  '--headless=new', '--disable-gpu', `--remote-debugging-port=${PORT}`,
  '--user-data-dir=' + process.env.TEMP + '/cdp-motion', 'about:blank',
], { stdio: 'ignore' });

async function siap() {
  for (let i = 0; i < 40; i++) {
    try { await fetch(`http://127.0.0.1:${PORT}/json/version`); return; } catch { await sleep(250); }
  }
  throw new Error('Chrome tidak siap');
}

await siap();
const tab = await (await fetch(`http://127.0.0.1:${PORT}/json/new?about:blank`, { method: 'PUT' })).json();
const ws = new WebSocket(tab.webSocketDebuggerUrl);
await new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej; });

let id = 0;
const tunggu = new Map();
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.id && tunggu.has(m.id)) { tunggu.get(m.id)(m); tunggu.delete(m.id); }
};
const cmd = (method, params = {}) => new Promise((res) => {
  const n = ++id;
  tunggu.set(n, res);
  ws.send(JSON.stringify({ id: n, method, params }));
});
const evaluate = async (expression) => {
  const r = await cmd('Runtime.evaluate', { expression, returnByValue: true });
  if (r.error) throw new Error(JSON.stringify(r.error));
  return JSON.parse(r.result.result.value);
};

await cmd('Runtime.enable');

const PILIH = `
(() => {
  const out = { lokasi: location.pathname, list: 0, stagger: 0, liftBeku: 0, liftHidup: 0 };
  for (const k of document.querySelectorAll('ul > li, ol > li, dl > div, .stagger > *')) {
    const cs = getComputedStyle(k);
    if (cs.animationName && cs.animationName !== 'none') out.list++;
    if (cs.animationDelay && cs.animationDelay !== '0s') out.stagger++;
  }
  for (const el of document.querySelectorAll('.lift')) {
    const cs = getComputedStyle(el);
    const identitas = cs.transform === 'none' || cs.transform === 'matrix(1, 0, 0, 1, 0, 0)';
    const dibekukan = cs.animationName !== 'none' && cs.animationFillMode.includes('both');
    if (dibekukan && identitas) out.liftBeku++; else out.liftHidup++;
  }
  return JSON.stringify(out);
})()`;

const hasil = [];
for (const path of HALAMAN) {
  await cmd('Page.navigate', { url: BASE + path });
  let terlihat = '';
  for (let i = 0; i < 40; i++) {
    await sleep(250);
    let v;
    try {
      v = await evaluate(PILIH);           // PILIH kini ikut mengembalikan lokasi
      terlihat = v.lokasi;
    } catch { continue; }                  // halaman belum siap
    if (v.list > 0 || i === 39) { hasil.push([path, terlihat, v]); break; }
  }
}

let gagal = 0;
for (const [p, lokasi, r] of hasil) {
  const salahHalaman = !lokasi.includes(p.split('?')[0]);
  console.log(`${p.padEnd(13)} -> ${lokasi.padEnd(18)} list=${r.list} stagger=${r.stagger} lift-beku=${r.liftBeku} lift-normal=${r.liftHidup}${salahHalaman ? '  <-- SALAH HALAMAN' : ''}`);
  if (salahHalaman) gagal++;
  gagal += r.liftBeku;
}
const totList = hasil.reduce((n, [, , r]) => n + r.list, 0);
const totStagger = hasil.reduce((n, [, , r]) => n + r.stagger, 0);
const totLift = hasil.reduce((n, [, , r]) => n + r.liftHidup, 0);
console.log(`\nTOTAL list=${totList} stagger=${totStagger} lift-hidup=${totLift}`);
const lulus = gagal === 0 && totList > 0 && totStagger > 0 && totLift > 0;
console.log(lulus ? 'LULUS' : 'GAGAL');
ws.close(); chrome.kill();
process.exit(lulus ? 0 : 1);
