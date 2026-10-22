package ui

import "strings"

// RenderSplashHTML genera el documento HTML5/CSS3/SVG con el logo animado de Cloudflare.
func RenderSplashHTML(authURL string) string {
	return strings.ReplaceAll(splashTemplate, "{{AUTH_URL}}", authURL)
}

const splashTemplate = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>CloudflareNetConnect — Asociar Cuenta</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; user-select: none; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: radial-gradient(circle at 50% 18%, #1e293b 0%, #0b0f19 72%);
    color: #f8fafc;
    height: 100vh;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: space-between;
    padding: 26px 24px 20px;
  }
  .logo-wrap {
    position: relative;
    width: 118px;
    height: 118px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-top: 6px;
  }
  .ring {
    position: absolute;
    border-radius: 50%;
    border: 2px solid rgba(244, 129, 32, 0.45);
    animation: pulseRing 2.6s cubic-bezier(0.215, 0.61, 0.355, 1) infinite;
  }
  .ring-1 { width: 84px; height: 84px; animation-delay: 0s; }
  .ring-2 { width: 84px; height: 84px; animation-delay: 0.9s; }
  .ring-3 { width: 84px; height: 84px; animation-delay: 1.8s; }
  @keyframes pulseRing {
    0% { transform: scale(0.85); opacity: 0.9; }
    100% { transform: scale(1.65); opacity: 0; }
  }
  .logo-core {
    width: 78px;
    height: 78px;
    border-radius: 50%;
    background: linear-gradient(145deg, rgba(244,129,32,0.18), rgba(250,173,63,0.06));
    border: 1px solid rgba(244,129,32,0.4);
    box-shadow: 0 0 30px rgba(244,129,32,0.28);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2;
    animation: floatCloud 3s ease-in-out infinite;
  }
  @keyframes floatCloud {
    0%, 100% { transform: translateY(0px); }
    50% { transform: translateY(-4px); }
  }
  .title-block { text-align: center; }
  h1 { font-size: 1.18rem; font-weight: 700; letter-spacing: -0.01em; color: #ffffff; }
  .subtitle { font-size: 0.82rem; color: #94a3b8; margin-top: 5px; line-height: 1.35; }
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 9px;
    margin-top: 12px;
    padding: 7px 15px;
    border-radius: 999px;
    background: rgba(15, 23, 42, 0.85);
    border: 1px solid rgba(244, 129, 32, 0.35);
    font-size: 0.8rem;
    font-weight: 500;
    color: #fdba74;
  }
  .spinner {
    width: 12px;
    height: 12px;
    border: 2px solid rgba(244,129,32,0.3);
    border-top-color: #f48120;
    border-radius: 50%;
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }
  .actions { width: 100%; display: flex; flex-direction: column; gap: 9px; }
  .btn-primary {
    width: 100%;
    padding: 12px 16px;
    border-radius: 10px;
    border: none;
    background: linear-gradient(135deg, #f48120 0%, #faad3f 100%);
    color: #0b0f19;
    font-weight: 700;
    font-size: 0.9rem;
    cursor: pointer;
    box-shadow: 0 6px 20px rgba(244, 129, 32, 0.32);
    transition: transform 0.12s ease, filter 0.15s ease;
  }
  .btn-primary:hover { filter: brightness(1.06); transform: translateY(-1px); }
  .btn-primary.copied {
    background: linear-gradient(135deg, #10b981 0%, #34d399 100%);
    color: #052e16;
    box-shadow: 0 6px 20px rgba(16, 185, 129, 0.32);
  }
  .row-btns { display: flex; gap: 8px; width: 100%; }
  .btn-sec {
    flex: 1;
    padding: 9px 10px;
    border-radius: 8px;
    border: 1px solid #334155;
    background: rgba(30, 41, 59, 0.7);
    color: #cbd5e1;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.15s;
  }
  .btn-sec:hover { background: rgba(51, 65, 85, 0.85); color: #fff; }
  .manual-box {
    display: none;
    width: 100%;
    gap: 6px;
    margin-top: 2px;
  }
  .manual-box input {
    flex: 1;
    padding: 8px 10px;
    border-radius: 7px;
    border: 1px solid #475569;
    background: #0f172a;
    color: #fff;
    font-size: 0.78rem;
  }
  .hint { font-size: 0.72rem; color: #64748b; text-align: center; }
</style>
</head>
<body>
  <div class="logo-wrap">
    <div class="ring ring-1"></div>
    <div class="ring ring-2"></div>
    <div class="ring ring-3"></div>
    <div class="logo-core">
      <svg width="46" height="46" viewBox="0 0 128 128" fill="none">
        <path d="M97.4 79.7c1.6-4.9 1-10.3-1.8-14.5-2.8-4.2-7.5-6.8-12.6-6.9l-9.9-.2c-.5 0-.9-.3-1.1-.7-.2-.4-.2-.9.1-1.3l4.9-8.3c2.4-4.1 1.7-9.4-1.7-12.7-3.4-3.3-8.7-3.9-12.8-1.3L22.6 58.6c-4.7 2.9-7.4 8.2-7.1 13.7.3 5.5 3.6 10.4 8.6 12.8l73.3-.2c.1-1.7-.1-3.5-.1-5.2z" fill="#F48120"/>
        <path d="M104.2 59.8c-.7 0-1.4.1-2.1.2-1.4-9.2-9.3-16.1-18.8-16.1-8.4 0-15.6 5.5-18.1 13.1-1.6-.7-3.4-1.1-5.3-1.1-7.2 0-13.1 5.9-13.1 13.1 0 1.2.2 2.4.5 3.5H24.6c-6.4 0-11.6 5.2-11.6 11.6S18.2 95.7 24.6 95.7h79.6c9.9 0 18-8.1 18-18s-8.1-17.9-18-17.9z" fill="#FAAD3F" fill-opacity="0.92"/>
      </svg>
    </div>
  </div>

  <div class="title-block">
    <h1>Vincular Cuenta Cloudflare</h1>
    <p class="subtitle">Copia el enlace de identificación y pégalo en el navegador donde tengas abierta tu cuenta.</p>
    <div class="status-pill" id="statusPill">
      <div class="spinner" id="spinIcon"></div>
      <span id="statusText">Esperando identificación...</span>
    </div>
  </div>

  <div class="actions">
    <button class="btn-primary" id="copyBtn" onclick="copyLink()">
      📋 Copiar Enlace de Identificación
    </button>
    <div class="row-btns">
      <button class="btn-sec" onclick="openBrowser()">🌐 Abrir navegador</button>
      <button class="btn-sec" onclick="toggleManual()">🔑 Token manual</button>
      <button class="btn-sec" onclick="cancelFlow()">✖ Cancelar</button>
    </div>
    <div class="manual-box" id="manualBox">
      <input type="password" id="tokenInput" placeholder="Pegar API Token..." />
      <button class="btn-sec" style="flex:0 0 74px;background:#f48120;color:#0b0f19;border:none" onclick="sendManual()">Vincular</button>
    </div>
  </div>

  <div class="hint">La ventana se cerrará sola al completar la autorización.</div>

<script>
const authURL = "{{AUTH_URL}}";
async function copyLink() {
  try { await navigator.clipboard.writeText(authURL); } catch(e) {}
  await fetch('/copy', { method: 'POST' });
  const btn = document.getElementById('copyBtn');
  btn.classList.add('copied');
  btn.innerText = '✅ ¡Enlace copiado! Pégalo en tu navegador';
  setTimeout(() => {
    btn.classList.remove('copied');
    btn.innerText = '📋 Copiar Enlace de Identificación';
  }, 3500);
}
async function openBrowser() {
  await fetch('/open-browser', { method: 'POST' });
}
function toggleManual() {
  const b = document.getElementById('manualBox');
  b.style.display = b.style.display === 'flex' ? 'none' : 'flex';
}
async function sendManual() {
  const t = document.getElementById('tokenInput').value.trim();
  if (!t) return;
  await fetch('/manual-token', { method: 'POST', body: JSON.stringify({token: t}) });
}
async function cancelFlow() {
  try { await fetch('/cancel', { method: 'POST' }); } catch(e) {}
  window.close();
}
setInterval(async () => {
  try {
    const r = await fetch('/status');
    const d = await r.json();
    if (d.done) {
      document.getElementById('spinIcon').style.display = 'none';
      const pill = document.getElementById('statusPill');
      pill.style.borderColor = '#10b981';
      pill.style.color = '#34d399';
      document.getElementById('statusText').innerText = '✅ ¡Cuenta ' + (d.account || '') + ' vinculada!';
      setTimeout(() => window.close(), 900);
    }
  } catch(e) {
    window.close();
  }
}, 750);
</script>
</body>
</html>`
