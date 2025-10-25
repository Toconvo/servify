(function(){
  const $ = (sel) => document.querySelector(sel);
  const $$ = (sel) => Array.from(document.querySelectorAll(sel));
  const API_V1 = '/api/v1';
  const API = '/api'; // 管理类 API

  function setActive(tab){
    $$('#main nav button');
    $$('.tab').forEach(el => el.classList.remove('active'));
    $(`section#${tab}`).classList.add('active');
    $$('nav button').forEach(b=>b.classList.toggle('active', b.dataset.tab===tab));
  }

  // 导航
  $$('nav button').forEach(b => b.addEventListener('click', () => setActive(b.dataset.tab)));

  // 简易 fetch 封装
  async function jget(url){
    const r = await fetch(url); if(!r.ok) throw new Error('HTTP '+r.status); return r.json();
  }
  async function jpost(url, data){
    const r = await fetch(url, { method: 'POST', headers: { 'Content-Type':'application/json' }, body: JSON.stringify(data||{}) });
    if(!r.ok) throw new Error('HTTP '+r.status); return r.json();
  }

  // 仪表板
  async function loadDashboard(){
    try {
      const platforms = await jget(`${API_V1}/messages/platforms`);
      $('#platforms_json').textContent = JSON.stringify(platforms, null, 2);
    } catch(e) {
      $('#platforms_json').textContent = '加载失败: '+e.message;
    }

    // 统计（增强服务端才有）
    try {
      const dash = await jget(`${API}/statistics/dashboard`);
      const d = dash.data || dash || {};
      $('#stat_sessions').textContent = d.total_sessions_today ?? '-';
      $('#stat_tickets_resolved').textContent = d.resolved_tickets_today ?? '-';
      $('#stat_agents_online').textContent = d.online_agents ?? '-';
    } catch(e) {
      // 忽略
    }
  }

  // 工单
  async function loadTickets(){
    try {
      const res = await jget(`${API}/tickets`);
      const list = res.data?.items || res.data || res || [];
      const tbody = $('#tbl_tickets tbody');
      tbody.innerHTML = '';
      list.forEach(t => {
        const tr = document.createElement('tr');
        tr.innerHTML = `<td>${t.id||''}</td><td>${t.title||''}</td><td>${t.status||''}</td><td>${t.priority||''}</td><td>${t.customer_id||''}</td><td><button data-id="${t.id}">详情</button></td>`;
        tbody.appendChild(tr);
      });
    } catch(e) {
      $('#tbl_tickets tbody').innerHTML = `<tr><td colspan="6">加载失败: ${e.message}</td></tr>`;
    }
  }

  $('#form_ticket')?.addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const fd = new FormData(ev.target);
    const data = Object.fromEntries(fd.entries());
    data.customer_id = data.customer_id ? parseInt(data.customer_id,10) : undefined;
    try {
      await jpost(`${API}/tickets`, data);
      ev.target.reset();
      await loadTickets();
      alert('创建成功');
    } catch(e){ alert('创建失败: '+e.message); }
  });

  // 客户
  async function loadCustomers(){
    try {
      const res = await jget(`${API}/customers`);
      const list = res.data?.items || res.data || res || [];
      const tbody = $('#tbl_customers tbody');
      tbody.innerHTML = '';
      list.forEach(c => {
        const tr = document.createElement('tr');
        tr.innerHTML = `<td>${c.id||''}</td><td>${c.name||''}</td><td>${c.email||''}</td><td>${(c.tags||[]).join?c.tags.join(', '):c.tags||''}</td>`;
        tbody.appendChild(tr);
      });
    } catch(e) {
      $('#tbl_customers tbody').innerHTML = `<tr><td colspan="4">加载失败: ${e.message}</td></tr>`;
    }
  }

  $('#form_customer')?.addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const fd = new FormData(ev.target);
    const data = Object.fromEntries(fd.entries());
    if (data.tags) data.tags = data.tags.split(',').map(s=>s.trim()).filter(Boolean);
    try { await jpost(`${API}/customers`, data); ev.target.reset(); await loadCustomers(); alert('创建成功'); }
    catch(e){ alert('创建失败: '+e.message); }
  });

  // 客服
  async function loadAgents(){
    try {
      const res = await jget(`${API}/agents/online`);
      const list = res.data || res || [];
      const tbody = $('#tbl_agents tbody');
      tbody.innerHTML = '';
      list.forEach(a => {
        const tr = document.createElement('tr');
        tr.innerHTML = `<td>${a.id||''}</td><td>${a.name||''}</td><td>${a.status||''}</td><td>${a.online? '是':'否'}</td>`;
        tbody.appendChild(tr);
      });
    } catch(e) {
      $('#tbl_agents tbody').innerHTML = `<tr><td colspan="4">加载失败: ${e.message}</td></tr>`;
    }
  }

  // AI 状态与测试
  async function loadAI(){
    try {
      const res = await jget(`${API_V1}/ai/status`);
      $('#ai_status').textContent = JSON.stringify(res, null, 2);
    } catch(e){ $('#ai_status').textContent = '加载失败: '+e.message; }
  }

  $('#btn_ai_query')?.addEventListener('click', async () => {
    const q = $('#ai_query').value;
    try {
      const res = await jpost(`${API_V1}/ai/query`, { query: q, session_id: 'admin_'+Date.now() });
      $('#ai_answer').textContent = JSON.stringify(res, null, 2);
    } catch(e){ $('#ai_answer').textContent = '失败: '+e.message; }
  });

  // 初始化
  loadDashboard();
  loadTickets();
  loadCustomers();
  loadAgents();
  loadAI();
})();
