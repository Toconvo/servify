(function(root){
  function $(sel, ctx){ return (ctx||document).querySelector(sel); }
  function el(tag, cls, text){ const e=document.createElement(tag); if(cls) e.className=cls; if(text) e.textContent=text; return e; }

  function createWidget(opts){
    opts = opts || {};
    const baseUrl = opts.baseUrl || (location.origin);
    const sessionId = opts.sessionId || ('w_'+Date.now());

    // ensure SDK available (UMD)
    var Servify = root.Servify;
    if (!Servify) { console.error('Servify SDK not found. Include /sdk/servify-sdk.umd.js first.'); return; }
    const client = Servify.createClient({ baseUrl, sessionId });

    const wrap = el('div', 'servify-widget');
    const btn = el('button', 'btn'); btn.innerHTML = '💬';
    const panel = el('div', 'panel');
    const header = el('header', '', 'Servify 在线客服');
    const status = el('div', 'status', '连接中...');
    const msgs = el('div', 'messages');
    const inputWrap = el('div', 'input');
    const input = el('input'); input.placeholder = '输入消息...';
    const sendBtn = el('button', '', '发送');

    inputWrap.appendChild(input); inputWrap.appendChild(sendBtn);
    panel.appendChild(header); panel.appendChild(status); panel.appendChild(msgs); panel.appendChild(inputWrap);
    wrap.appendChild(btn); wrap.appendChild(panel);
    document.body.appendChild(wrap);

    function addMsg(role, content){
      const m = el('div', 'msg '+role);
      const b = el('div', 'bubble', content);
      m.appendChild(b);
      msgs.appendChild(m);
      msgs.scrollTop = msgs.scrollHeight;
    }

    btn.addEventListener('click', () => panel.classList.toggle('open'));
    sendBtn.addEventListener('click', () => { if (!input.value.trim()) return; client.sendMessage(input.value.trim()); addMsg('user', input.value.trim()); input.value=''; });
    input.addEventListener('keydown', (e)=>{ if(e.key==='Enter'){ e.preventDefault(); sendBtn.click(); }});

    client.on('status', (s) => status.textContent = '状态: '+s);
    client.on('ai', (m) => addMsg('ai', m.content || ''));
    client.on('message', (m) => { if (m && m.type==='text-message') addMsg('user', m.data?.content||''); });
    client.connect();

    return { mount: wrap, client };
  }

  root.ServifyWidget = { create: createWidget };

  // auto init by <script data-servify-widget> if present
  if (document.currentScript && document.currentScript.hasAttribute('data-servify-widget')){
    var baseUrl = document.currentScript.getAttribute('data-base-url') || location.origin;
    var sessionId = document.currentScript.getAttribute('data-session-id') || '';
    createWidget({ baseUrl: baseUrl, sessionId: sessionId });
  }
})(this);
