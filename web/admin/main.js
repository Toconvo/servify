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
  let dashboardCharts = {
    ticketTrend: null,
    satisfactionDistribution: null,
    agentWorkload: null,
    platformStats: null
  };

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

      // 加载满意度统计
      await loadDashboardSatisfactionStats();
    } catch(e) {
      // 忽略
    }

    // 初始化图表
    await initializeDashboardCharts();
  }

  async function loadDashboardSatisfactionStats() {
    try {
      const satisfactionStats = await jget(`${API}/satisfactions/stats`);
      const avgRating = satisfactionStats.average_rating ? satisfactionStats.average_rating.toFixed(1) : '-';
      $('#stat_satisfaction_avg').textContent = avgRating + (avgRating !== '-' ? ' ★' : '');
    } catch(e) {
      $('#stat_satisfaction_avg').textContent = '-';
    }
  }

  async function initializeDashboardCharts() {
    // 1. 工单趋势图
    await initTicketTrendChart();

    // 2. 满意度分布图
    await initSatisfactionDistributionChart();

    // 3. 客服工作负载图
    await initAgentWorkloadChart();

    // 4. 平台接入统计图
    await initPlatformStatsChart();
  }

  async function initTicketTrendChart() {
    if (!window.echarts) return;

    const chartDom = document.getElementById('ticket-trend-chart');
    if (!chartDom) return;

    if (dashboardCharts.ticketTrend) {
      dashboardCharts.ticketTrend.dispose();
    }

    dashboardCharts.ticketTrend = echarts.init(chartDom);

    try {
      // 获取最近7天的工单统计数据
      const endDate = new Date();
      const startDate = new Date();
      startDate.setDate(endDate.getDate() - 6);

      const dateRange = [];
      const createdData = [];
      const resolvedData = [];

      for (let d = new Date(startDate); d <= endDate; d.setDate(d.getDate() + 1)) {
        const dateStr = d.toISOString().split('T')[0];
        dateRange.push(d.getMonth() + 1 + '/' + d.getDate());

        // 这里可以调用真实API获取数据，暂时使用模拟数据
        createdData.push(Math.floor(Math.random() * 50) + 10);
        resolvedData.push(Math.floor(Math.random() * 45) + 5);
      }

      const option = {
        tooltip: {
          trigger: 'axis',
          axisPointer: {
            type: 'cross'
          }
        },
        legend: {
          data: ['新建工单', '解决工单']
        },
        grid: {
          left: '3%',
          right: '4%',
          bottom: '3%',
          containLabel: true
        },
        xAxis: {
          type: 'category',
          data: dateRange
        },
        yAxis: {
          type: 'value'
        },
        series: [
          {
            name: '新建工单',
            type: 'line',
            data: createdData,
            smooth: true,
            itemStyle: {
              color: '#4299e1'
            }
          },
          {
            name: '解决工单',
            type: 'line',
            data: resolvedData,
            smooth: true,
            itemStyle: {
              color: '#48bb78'
            }
          }
        ]
      };

      dashboardCharts.ticketTrend.setOption(option);
    } catch (e) {
      console.error('Failed to init ticket trend chart:', e);
    }
  }

  async function initSatisfactionDistributionChart() {
    if (!window.echarts) return;

    const chartDom = document.getElementById('satisfaction-distribution-chart');
    if (!chartDom) return;

    if (dashboardCharts.satisfactionDistribution) {
      dashboardCharts.satisfactionDistribution.dispose();
    }

    dashboardCharts.satisfactionDistribution = echarts.init(chartDom);

    try {
      const satisfactionStats = await jget(`${API}/satisfactions/stats`);
      const distribution = satisfactionStats.rating_distribution || {};

      const data = [
        { value: distribution[5] || 0, name: '5星 (非常满意)' },
        { value: distribution[4] || 0, name: '4星 (满意)' },
        { value: distribution[3] || 0, name: '3星 (一般)' },
        { value: distribution[2] || 0, name: '2星 (不满意)' },
        { value: distribution[1] || 0, name: '1星 (非常不满意)' }
      ];

      const option = {
        tooltip: {
          trigger: 'item',
          formatter: '{a} <br/>{b}: {c} ({d}%)'
        },
        legend: {
          orient: 'vertical',
          left: 'left',
          textStyle: {
            fontSize: 12
          }
        },
        series: [
          {
            name: '满意度评分',
            type: 'pie',
            radius: ['40%', '70%'],
            center: ['65%', '50%'],
            avoidLabelOverlap: false,
            label: {
              show: false,
              position: 'center'
            },
            emphasis: {
              label: {
                show: true,
                fontSize: '18',
                fontWeight: 'bold'
              }
            },
            labelLine: {
              show: false
            },
            data: data,
            itemStyle: {
              color: function(params) {
                const colors = ['#f56565', '#fd9803', '#ecc94b', '#68d391', '#48bb78'];
                return colors[params.dataIndex];
              }
            }
          }
        ]
      };

      dashboardCharts.satisfactionDistribution.setOption(option);
    } catch (e) {
      console.error('Failed to init satisfaction distribution chart:', e);
    }
  }

  async function initAgentWorkloadChart() {
    if (!window.echarts) return;

    const chartDom = document.getElementById('agent-workload-chart');
    if (!chartDom) return;

    if (dashboardCharts.agentWorkload) {
      dashboardCharts.agentWorkload.dispose();
    }

    dashboardCharts.agentWorkload = echarts.init(chartDom);

    try {
      // 获取客服工作负载数据
      const agentsRes = await jget(`${API}/agents`);
      const agents = agentsRes.data || [];

      const names = agents.slice(0, 10).map(agent => agent.user?.name || `客服${agent.id}`);
      const loads = agents.slice(0, 10).map(agent => agent.current_load || 0);
      const maxLoads = agents.slice(0, 10).map(agent => agent.max_concurrent || 5);

      const option = {
        tooltip: {
          trigger: 'axis',
          axisPointer: {
            type: 'shadow'
          }
        },
        legend: {
          data: ['当前工单', '最大容量']
        },
        grid: {
          left: '3%',
          right: '4%',
          bottom: '3%',
          containLabel: true
        },
        xAxis: {
          type: 'value'
        },
        yAxis: {
          type: 'category',
          data: names
        },
        series: [
          {
            name: '当前工单',
            type: 'bar',
            data: loads,
            itemStyle: {
              color: '#4299e1'
            }
          },
          {
            name: '最大容量',
            type: 'bar',
            data: maxLoads,
            itemStyle: {
              color: '#e2e8f0'
            }
          }
        ]
      };

      dashboardCharts.agentWorkload.setOption(option);
    } catch (e) {
      console.error('Failed to init agent workload chart:', e);
    }
  }

  async function initPlatformStatsChart() {
    if (!window.echarts) return;

    const chartDom = document.getElementById('platform-stats-chart');
    if (!chartDom) return;

    if (dashboardCharts.platformStats) {
      dashboardCharts.platformStats.dispose();
    }

    dashboardCharts.platformStats = echarts.init(chartDom);

    try {
      const platforms = await jget(`${API_V1}/messages/platforms`);

      const data = Object.entries(platforms).map(([platform, count]) => ({
        name: platform,
        value: count
      }));

      const option = {
        tooltip: {
          trigger: 'item'
        },
        legend: {
          top: '5%',
          left: 'center'
        },
        series: [
          {
            name: '平台接入',
            type: 'pie',
            radius: ['40%', '70%'],
            avoidLabelOverlap: false,
            label: {
              show: false,
              position: 'center'
            },
            emphasis: {
              label: {
                show: true,
                fontSize: '18',
                fontWeight: 'bold'
              }
            },
            labelLine: {
              show: false
            },
            data: data
          }
        ]
      };

      dashboardCharts.platformStats.setOption(option);
    } catch (e) {
      console.error('Failed to init platform stats chart:', e);
    }
  }

  // 工单
  let ticketCharts = {
    status: null,
    priority: null
  };

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

      // 初始化工单图表
      await initializeTicketCharts(list);
    } catch(e) {
      $('#tbl_tickets tbody').innerHTML = `<tr><td colspan="6">加载失败: ${e.message}</td></tr>`;
    }
  }

  async function initializeTicketCharts(ticketData = null) {
    if (!window.echarts) return;

    if (!ticketData) {
      try {
        const res = await jget(`${API}/tickets`);
        ticketData = res.data?.items || res.data || res || [];
      } catch (e) {
        console.error('Failed to load ticket data for charts:', e);
        return;
      }
    }

    await initTicketStatusChart(ticketData);
    await initTicketPriorityChart(ticketData);
  }

  async function initTicketStatusChart(ticketData) {
    const chartDom = document.getElementById('ticket-status-chart');
    if (!chartDom) return;

    if (ticketCharts.status) {
      ticketCharts.status.dispose();
    }

    ticketCharts.status = echarts.init(chartDom);

    // 统计状态分布
    const statusCount = {};
    ticketData.forEach(ticket => {
      const status = ticket.status || 'unknown';
      statusCount[status] = (statusCount[status] || 0) + 1;
    });

    // 如果没有数据，使用模拟数据
    if (Object.keys(statusCount).length === 0) {
      statusCount.open = 15;
      statusCount.assigned = 12;
      statusCount.in_progress = 8;
      statusCount.resolved = 20;
      statusCount.closed = 18;
    }

    const statusNames = {
      open: '新建',
      assigned: '已分配',
      in_progress: '处理中',
      resolved: '已解决',
      closed: '已关闭'
    };

    const data = Object.entries(statusCount).map(([status, count]) => ({
      name: statusNames[status] || status,
      value: count
    }));

    const option = {
      tooltip: {
        trigger: 'item',
        formatter: '{a} <br/>{b}: {c} ({d}%)'
      },
      legend: {
        orient: 'vertical',
        left: 'left',
        textStyle: {
          fontSize: 12
        }
      },
      series: [
        {
          name: '工单状态',
          type: 'pie',
          radius: ['30%', '70%'],
          center: ['60%', '50%'],
          data: data,
          emphasis: {
            itemStyle: {
              shadowBlur: 10,
              shadowOffsetX: 0,
              shadowColor: 'rgba(0, 0, 0, 0.5)'
            }
          },
          itemStyle: {
            color: function(params) {
              const colors = ['#4299e1', '#48bb78', '#ecc94b', '#9f7aea', '#38b2ac'];
              return colors[params.dataIndex % colors.length];
            }
          }
        }
      ]
    };

    ticketCharts.status.setOption(option);
  }

  async function initTicketPriorityChart(ticketData) {
    const chartDom = document.getElementById('ticket-priority-chart');
    if (!chartDom) return;

    if (ticketCharts.priority) {
      ticketCharts.priority.dispose();
    }

    ticketCharts.priority = echarts.init(chartDom);

    // 统计优先级分布
    const priorityCount = {};
    ticketData.forEach(ticket => {
      const priority = ticket.priority || 'normal';
      priorityCount[priority] = (priorityCount[priority] || 0) + 1;
    });

    // 如果没有数据，使用模拟数据
    if (Object.keys(priorityCount).length === 0) {
      priorityCount.low = 25;
      priorityCount.normal = 30;
      priorityCount.high = 10;
      priorityCount.urgent = 5;
    }

    const priorityNames = {
      low: '低',
      normal: '正常',
      high: '高',
      urgent: '紧急'
    };

    const categories = ['low', 'normal', 'high', 'urgent'];
    const data = categories.map(priority => priorityCount[priority] || 0);
    const names = categories.map(priority => priorityNames[priority]);

    const option = {
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'shadow'
        }
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        data: names
      },
      yAxis: {
        type: 'value'
      },
      series: [
        {
          name: '工单数量',
          type: 'bar',
          data: data,
          itemStyle: {
            color: function(params) {
              const colors = ['#48bb78', '#4299e1', '#ecc94b', '#f56565'];
              return colors[params.dataIndex];
            }
          },
          emphasis: {
            itemStyle: {
              shadowBlur: 10,
              shadowOffsetX: 0,
              shadowColor: 'rgba(0, 0, 0, 0.5)'
            }
          }
        }
      ]
    };

    ticketCharts.priority.setOption(option);
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

  // 满意度评价管理
  let satisfactionCurrentPage = 1;
  let satisfactionPageSize = 20;
  let satisfactionCharts = {
    ratingPie: null,
    trend: null,
    category: null
  };

  async function loadSatisfactions(page = 1, filters = {}) {
    try {
      const params = new URLSearchParams({
        page: page.toString(),
        page_size: satisfactionPageSize.toString(),
        ...filters
      });

      const res = await jget(`${API}/satisfactions?${params}`);
      const tbody = $('#tbl_satisfactions tbody');
      tbody.innerHTML = '';

      if (res.data && res.data.length > 0) {
        res.data.forEach(satisfaction => {
          const tr = document.createElement('tr');

          // 格式化评分显示
          const ratingStars = '★'.repeat(satisfaction.rating) + '☆'.repeat(5 - satisfaction.rating);

          // 格式化日期
          const createdAt = new Date(satisfaction.created_at).toLocaleString('zh-CN');

          tr.innerHTML = `
            <td>${satisfaction.id}</td>
            <td>#${satisfaction.ticket_id}</td>
            <td>${satisfaction.customer?.name || '-'}</td>
            <td>${satisfaction.agent?.name || '系统处理'}</td>
            <td><span style="color: #f6ad55;">${ratingStars}</span> (${satisfaction.rating})</td>
            <td>${satisfaction.category || '-'}</td>
            <td title="${satisfaction.comment || ''}">${satisfaction.comment ? satisfaction.comment.substring(0, 30) + (satisfaction.comment.length > 30 ? '...' : '') : '-'}</td>
            <td>${createdAt}</td>
            <td>
              <button onclick="viewSatisfactionDetail(${satisfaction.id})" style="margin-right: 5px; padding: 4px 8px; font-size: 12px;">详情</button>
              <button onclick="deleteSatisfaction(${satisfaction.id})" style="background: #e53e3e; color: white; padding: 4px 8px; font-size: 12px; border: none; border-radius: 4px; cursor: pointer;">删除</button>
            </td>
          `;
          tbody.appendChild(tr);
        });
      } else {
        tbody.innerHTML = '<tr><td colspan="9" style="text-align: center; color: #718096;">暂无满意度评价数据</td></tr>';
      }

      // 更新分页信息
      const totalPages = Math.ceil(res.total / satisfactionPageSize);
      $('#satisfaction_page_info').textContent = `第${page}页 / 共${totalPages}页 (${res.total}条记录)`;
      $('#btn_satisfaction_prev').disabled = page <= 1;
      $('#btn_satisfaction_next').disabled = page >= totalPages;

      satisfactionCurrentPage = page;
    } catch (e) {
      $('#tbl_satisfactions tbody').innerHTML = `<tr><td colspan="9" style="text-align: center; color: #e53e3e;">加载失败: ${e.message}</td></tr>`;
    }
  }

  async function loadSatisfactionStats() {
    try {
      const res = await jget(`${API}/satisfactions/stats`);

      $('#satisfaction_total_count').textContent = res.total_ratings || 0;
      $('#satisfaction_avg_rating').textContent = res.average_rating ? res.average_rating.toFixed(2) : '-';

      // 计算5星占比
      const fiveStarCount = res.rating_distribution?.[5] || 0;
      const fiveStarRate = res.total_ratings > 0 ? ((fiveStarCount / res.total_ratings) * 100).toFixed(1) : 0;
      $('#satisfaction_five_star_rate').textContent = fiveStarRate + '%';

      // 计算低评分工单（1-2星）
      const lowRatingCount = (res.rating_distribution?.[1] || 0) + (res.rating_distribution?.[2] || 0);
      $('#satisfaction_low_rating_count').textContent = lowRatingCount;

      // 初始化满意度图表
      await initializeSatisfactionCharts(res);
    } catch (e) {
      console.error('Failed to load satisfaction stats:', e);
    }
  }

  async function initializeSatisfactionCharts(statsData = null) {
    if (!statsData) {
      try {
        statsData = await jget(`${API}/satisfactions/stats`);
      } catch (e) {
        console.error('Failed to load satisfaction stats for charts:', e);
        return;
      }
    }

    // 1. 评分分布饼图
    await initSatisfactionRatingPieChart(statsData);

    // 2. 满意度趋势图
    await initSatisfactionTrendChart(statsData);

    // 3. 按分类统计柱状图
    await initSatisfactionCategoryChart(statsData);
  }

  async function initSatisfactionRatingPieChart(statsData) {
    if (!window.echarts) return;

    const chartDom = document.getElementById('satisfaction-rating-pie-chart');
    if (!chartDom) return;

    if (satisfactionCharts.ratingPie) {
      satisfactionCharts.ratingPie.dispose();
    }

    satisfactionCharts.ratingPie = echarts.init(chartDom);

    const distribution = statsData.rating_distribution || {};

    const data = [
      { value: distribution[5] || 0, name: '5星 (非常满意)' },
      { value: distribution[4] || 0, name: '4星 (满意)' },
      { value: distribution[3] || 0, name: '3星 (一般)' },
      { value: distribution[2] || 0, name: '2星 (不满意)' },
      { value: distribution[1] || 0, name: '1星 (非常不满意)' }
    ];

    const option = {
      tooltip: {
        trigger: 'item',
        formatter: function(params) {
          const total = data.reduce((sum, item) => sum + item.value, 0);
          const percentage = total > 0 ? ((params.value / total) * 100).toFixed(1) : 0;
          return `${params.name}<br/>${params.value}条 (${percentage}%)`;
        }
      },
      legend: {
        orient: 'vertical',
        left: 'left',
        textStyle: {
          fontSize: 12
        }
      },
      series: [
        {
          name: '满意度评分分布',
          type: 'pie',
          radius: ['40%', '70%'],
          center: ['60%', '50%'],
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 10,
            borderColor: '#fff',
            borderWidth: 2
          },
          label: {
            show: false,
            position: 'center'
          },
          emphasis: {
            label: {
              show: true,
              fontSize: '18',
              fontWeight: 'bold'
            }
          },
          labelLine: {
            show: false
          },
          data: data,
          itemStyle: {
            color: function(params) {
              const colors = ['#48bb78', '#68d391', '#ecc94b', '#fd9803', '#f56565'];
              return colors[params.dataIndex];
            }
          }
        }
      ]
    };

    satisfactionCharts.ratingPie.setOption(option);
  }

  async function initSatisfactionTrendChart(statsData) {
    if (!window.echarts) return;

    const chartDom = document.getElementById('satisfaction-trend-chart');
    if (!chartDom) return;

    if (satisfactionCharts.trend) {
      satisfactionCharts.trend.dispose();
    }

    satisfactionCharts.trend = echarts.init(chartDom);

    const trendData = statsData.trend_data || [];

    // 如果没有趋势数据，生成模拟数据
    let dates, counts, avgRatings;
    if (trendData.length === 0) {
      // 生成最近7天的模拟数据
      dates = [];
      counts = [];
      avgRatings = [];

      for (let i = 6; i >= 0; i--) {
        const date = new Date();
        date.setDate(date.getDate() - i);
        dates.push((date.getMonth() + 1) + '/' + date.getDate());
        counts.push(Math.floor(Math.random() * 20) + 5);
        avgRatings.push((Math.random() * 2 + 3).toFixed(1)); // 3-5星之间
      }
    } else {
      dates = trendData.map(item => new Date(item.date).toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' }));
      counts = trendData.map(item => item.count);
      avgRatings = trendData.map(item => item.average_rating.toFixed(1));
    }

    const option = {
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross'
        }
      },
      legend: {
        data: ['评价数量', '平均评分']
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        data: dates
      },
      yAxis: [
        {
          type: 'value',
          name: '评价数量',
          min: 0,
          position: 'left',
          axisLine: {
            show: true,
            lineStyle: {
              color: '#4299e1'
            }
          }
        },
        {
          type: 'value',
          name: '平均评分',
          min: 1,
          max: 5,
          position: 'right',
          axisLine: {
            show: true,
            lineStyle: {
              color: '#48bb78'
            }
          }
        }
      ],
      series: [
        {
          name: '评价数量',
          type: 'bar',
          data: counts,
          itemStyle: {
            color: '#4299e1'
          }
        },
        {
          name: '平均评分',
          type: 'line',
          yAxisIndex: 1,
          data: avgRatings,
          smooth: true,
          itemStyle: {
            color: '#48bb78'
          },
          lineStyle: {
            width: 3
          }
        }
      ]
    };

    satisfactionCharts.trend.setOption(option);
  }

  async function initSatisfactionCategoryChart(statsData) {
    if (!window.echarts) return;

    const chartDom = document.getElementById('satisfaction-category-chart');
    if (!chartDom) return;

    if (satisfactionCharts.category) {
      satisfactionCharts.category.dispose();
    }

    satisfactionCharts.category = echarts.init(chartDom);

    const categoryStats = statsData.category_stats || {};

    const categories = Object.keys(categoryStats);
    const counts = categories.map(cat => categoryStats[cat].count || 0);
    const avgRatings = categories.map(cat => categoryStats[cat].average_rating || 0);

    // 如果没有数据，使用模拟数据
    if (categories.length === 0) {
      categories.push('整体满意度', '服务质量', '响应速度', '解决质量');
      counts.push(25, 18, 20, 15);
      avgRatings.push(4.2, 4.5, 3.8, 4.1);
    }

    const option = {
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'shadow'
        }
      },
      legend: {
        data: ['评价数量', '平均评分']
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        data: categories.map(cat => {
          const categoryNames = {
            overall: '整体满意度',
            service_quality: '服务质量',
            response_time: '响应速度',
            resolution_quality: '解决质量'
          };
          return categoryNames[cat] || cat;
        })
      },
      yAxis: [
        {
          type: 'value',
          name: '评价数量',
          min: 0,
          position: 'left',
          axisLine: {
            show: true,
            lineStyle: {
              color: '#4299e1'
            }
          }
        },
        {
          type: 'value',
          name: '平均评分',
          min: 1,
          max: 5,
          position: 'right',
          axisLine: {
            show: true,
            lineStyle: {
              color: '#48bb78'
            }
          }
        }
      ],
      series: [
        {
          name: '评价数量',
          type: 'bar',
          data: counts,
          itemStyle: {
            color: '#4299e1'
          }
        },
        {
          name: '平均评分',
          type: 'line',
          yAxisIndex: 1,
          data: avgRatings,
          smooth: true,
          itemStyle: {
            color: '#48bb78'
          },
          lineStyle: {
            width: 3
          },
          symbol: 'circle',
          symbolSize: 8
        }
      ]
    };

    satisfactionCharts.category.setOption(option);
  }

  // 筛选功能
  $('#btn_satisfaction_filter')?.addEventListener('click', () => {
    const filters = {};

    const rating = $('#satisfaction_filter_rating').value;
    if (rating) {
      filters.rating = rating;
    }

    const dateFrom = $('#satisfaction_filter_date_from').value;
    if (dateFrom) {
      filters.date_from = dateFrom;
    }

    const dateTo = $('#satisfaction_filter_date_to').value;
    if (dateTo) {
      filters.date_to = dateTo;
    }

    loadSatisfactions(1, filters);
  });

  // 统计报告功能
  $('#btn_satisfaction_stats')?.addEventListener('click', async () => {
    try {
      await loadSatisfactionStats();
      alert('统计数据已刷新');
    } catch (e) {
      alert('刷新统计失败: ' + e.message);
    }
  });

  // 分页功能
  $('#btn_satisfaction_prev')?.addEventListener('click', () => {
    if (satisfactionCurrentPage > 1) {
      loadSatisfactions(satisfactionCurrentPage - 1);
    }
  });

  $('#btn_satisfaction_next')?.addEventListener('click', () => {
    loadSatisfactions(satisfactionCurrentPage + 1);
  });

  // 全局函数，供HTML调用
  window.viewSatisfactionDetail = async (id) => {
    try {
      const satisfaction = await jget(`${API}/satisfactions/${id}`);

      const ratingStars = '★'.repeat(satisfaction.rating) + '☆'.repeat(5 - satisfaction.rating);
      const createdAt = new Date(satisfaction.created_at).toLocaleString('zh-CN');

      const details = `
满意度评价详情

评价ID: ${satisfaction.id}
工单: #${satisfaction.ticket_id} - ${satisfaction.ticket?.title || ''}
客户: ${satisfaction.customer?.name || '-'}
客服: ${satisfaction.agent?.name || '系统处理'}
评分: ${ratingStars} (${satisfaction.rating}/5星)
分类: ${satisfaction.category || '-'}
评论: ${satisfaction.comment || '无评论'}
创建时间: ${createdAt}
      `.trim();

      alert(details);
    } catch (e) {
      alert('获取详情失败: ' + e.message);
    }
  };

  window.deleteSatisfaction = async (id) => {
    if (!confirm('确定要删除这条满意度评价吗？此操作不可恢复。')) {
      return;
    }

    try {
      await fetch(`${API}/satisfactions/${id}`, { method: 'DELETE' });
      alert('删除成功');
      loadSatisfactions(satisfactionCurrentPage);
      loadSatisfactionStats(); // 刷新统计
    } catch (e) {
      alert('删除失败: ' + e.message);
    }
  };

  // 初始化
  loadDashboard();
  loadTickets();
  loadCustomers();
  loadAgents();
  loadAI();

  // 导航切换时加载满意度数据
  const originalSetActive = setActive;
  window.setActive = function(tab) {
    originalSetActive(tab);
    if (tab === 'satisfaction') {
      loadSatisfactions();
      loadSatisfactionStats();
    } else if (tab === 'dashboard') {
      // 重新初始化仪表板图表
      setTimeout(() => {
        initializeDashboardCharts();
      }, 100);
    } else if (tab === 'tickets') {
      // 重新初始化工单图表
      setTimeout(() => {
        initializeTicketCharts();
      }, 100);
    }
  };
  setActive = window.setActive;

  // 窗口大小变化时重新调整图表大小
  window.addEventListener('resize', () => {
    // 延迟执行，避免频繁调用
    clearTimeout(window.resizeTimeout);
    window.resizeTimeout = setTimeout(() => {
      // 调整仪表板图表大小
      Object.values(dashboardCharts).forEach(chart => {
        if (chart && typeof chart.resize === 'function') {
          chart.resize();
        }
      });

      // 调整满意度图表大小
      Object.values(satisfactionCharts).forEach(chart => {
        if (chart && typeof chart.resize === 'function') {
          chart.resize();
        }
      });

      // 调整工单图表大小
      Object.values(ticketCharts).forEach(chart => {
        if (chart && typeof chart.resize === 'function') {
          chart.resize();
        }
      });
    }, 300);
  });

  // === SLA 管理功能 ===
  let slaCurrentConfigPage = 1;
  let slaCurrentViolationPage = 1;
  let slaPageSize = 20;
  let slaCharts = {
    complianceTrend: null,
    violationType: null,
    violationPriority: null
  };
  let slaConfigEditId = null;

  // SLA配置管理
  async function loadSLAConfigs(page = 1, filters = {}) {
    try {
      const params = new URLSearchParams({
        page: page.toString(),
        page_size: slaPageSize.toString(),
        ...filters
      });

      const res = await jget(`${API}/sla/configs?${params}`);
      const tbody = $('#tbl_sla_configs tbody');
      tbody.innerHTML = '';

      if (res.data && res.data.length > 0) {
        res.data.forEach(config => {
          const tr = document.createElement('tr');

          const priorityBadge = `<span class="priority-badge priority-${config.priority}">${config.priority}</span>`;
          const statusBadge = config.active ? '<span class="status-active">启用</span>' : '<span class="status-inactive">禁用</span>';
          const businessHours = config.business_hours_only ? '是' : '否';

          tr.innerHTML = `
            <td>${config.id}</td>
            <td>${config.name}</td>
            <td>${priorityBadge}</td>
            <td>${config.first_response_time}</td>
            <td>${config.resolution_time}</td>
            <td>${config.escalation_time}</td>
            <td>${businessHours}</td>
            <td>${statusBadge}</td>
            <td>
              <button onclick="editSLAConfig(${config.id})" style="margin-right: 5px; padding: 4px 8px; font-size: 12px;">编辑</button>
              <button onclick="deleteSLAConfig(${config.id})" style="background: #e53e3e; color: white; padding: 4px 8px; font-size: 12px; border: none; border-radius: 4px; cursor: pointer;">删除</button>
            </td>
          `;
          tbody.appendChild(tr);
        });
      } else {
        tbody.innerHTML = '<tr><td colspan="9" style="text-align: center; color: #718096;">暂无SLA配置数据</td></tr>';
      }

      // 更新分页信息
      const totalPages = Math.ceil(res.total / slaPageSize);
      $('#sla_configs_page_info').textContent = `第${page}页 / 共${totalPages}页 (${res.total}条记录)`;
      $('#btn_sla_configs_prev').disabled = page <= 1;
      $('#btn_sla_configs_next').disabled = page >= totalPages;

      slaCurrentConfigPage = page;
    } catch (e) {
      $('#tbl_sla_configs tbody').innerHTML = `<tr><td colspan="9" style="text-align: center; color: #e53e3e;">加载失败: ${e.message}</td></tr>`;
    }
  }

  // 加载SLA统计数据
  async function loadSLAStats() {
    try {
      const stats = await jget(`${API}/sla/stats`);

      $('#sla_total_configs').textContent = stats.total_configs || 0;
      $('#sla_active_configs').textContent = stats.active_configs || 0;
      $('#sla_compliance_rate').textContent = (stats.compliance_rate || 0).toFixed(1) + '%';
      $('#sla_unresolved_violations').textContent = stats.unresolved_violations || 0;

      // 初始化SLA图表
      await initializeSLACharts(stats);
    } catch (e) {
      console.error('Failed to load SLA stats:', e);
    }
  }

  // 初始化SLA图表
  async function initializeSLACharts(statsData) {
    if (!window.echarts) return;

    await initSLAComplianceTrendChart(statsData);
    await initSLAViolationTypeChart(statsData);
    await initSLAViolationPriorityChart(statsData);
  }

  // SLA合规趋势图
  async function initSLAComplianceTrendChart(statsData) {
    const chartDom = document.getElementById('sla-compliance-trend-chart');
    if (!chartDom) return;

    if (slaCharts.complianceTrend) {
      slaCharts.complianceTrend.dispose();
    }

    slaCharts.complianceTrend = echarts.init(chartDom);

    const trendData = statsData.trend_data || [];

    // 如果没有数据，生成模拟数据
    let dates, totalTickets, violations, complianceRates;
    if (trendData.length === 0) {
      dates = [];
      totalTickets = [];
      violations = [];
      complianceRates = [];

      for (let i = 6; i >= 0; i--) {
        const date = new Date();
        date.setDate(date.getDate() - i);
        dates.push((date.getMonth() + 1) + '/' + date.getDate());

        const tickets = Math.floor(Math.random() * 50) + 10;
        const viol = Math.floor(Math.random() * 5);
        totalTickets.push(tickets);
        violations.push(viol);
        complianceRates.push(tickets > 0 ? ((tickets - viol) / tickets * 100).toFixed(1) : 100);
      }
    } else {
      dates = trendData.map(item => new Date(item.date).toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' }));
      totalTickets = trendData.map(item => item.total_tickets);
      violations = trendData.map(item => item.violations);
      complianceRates = trendData.map(item => item.compliance_rate.toFixed(1));
    }

    const option = {
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross'
        }
      },
      legend: {
        data: ['工单总数', '违约数量', '合规率']
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        data: dates
      },
      yAxis: [
        {
          type: 'value',
          name: '数量',
          min: 0,
          position: 'left'
        },
        {
          type: 'value',
          name: '合规率(%)',
          min: 0,
          max: 100,
          position: 'right'
        }
      ],
      series: [
        {
          name: '工单总数',
          type: 'bar',
          data: totalTickets,
          itemStyle: {
            color: '#4299e1'
          }
        },
        {
          name: '违约数量',
          type: 'bar',
          data: violations,
          itemStyle: {
            color: '#f56565'
          }
        },
        {
          name: '合规率',
          type: 'line',
          yAxisIndex: 1,
          data: complianceRates,
          smooth: true,
          itemStyle: {
            color: '#48bb78'
          },
          lineStyle: {
            width: 3
          }
        }
      ]
    };

    slaCharts.complianceTrend.setOption(option);
  }

  // 违约类型分布图
  async function initSLAViolationTypeChart(statsData) {
    const chartDom = document.getElementById('sla-violation-type-chart');
    if (!chartDom) return;

    if (slaCharts.violationType) {
      slaCharts.violationType.dispose();
    }

    slaCharts.violationType = echarts.init(chartDom);

    const violationsByType = statsData.violations_by_type || {};

    const data = Object.entries(violationsByType).map(([type, count]) => ({
      name: type === 'first_response' ? '首次响应超时' : '解决时间超时',
      value: count
    }));

    // 如果没有数据，使用模拟数据
    if (data.length === 0) {
      data.push(
        { name: '首次响应超时', value: 15 },
        { name: '解决时间超时', value: 8 }
      );
    }

    const option = {
      tooltip: {
        trigger: 'item',
        formatter: '{a} <br/>{b}: {c} ({d}%)'
      },
      legend: {
        orient: 'vertical',
        left: 'left'
      },
      series: [
        {
          name: '违约类型',
          type: 'pie',
          radius: ['40%', '70%'],
          center: ['60%', '50%'],
          data: data,
          emphasis: {
            itemStyle: {
              shadowBlur: 10,
              shadowOffsetX: 0,
              shadowColor: 'rgba(0, 0, 0, 0.5)'
            }
          },
          itemStyle: {
            color: function(params) {
              const colors = ['#f56565', '#fd9803'];
              return colors[params.dataIndex % colors.length];
            }
          }
        }
      ]
    };

    slaCharts.violationType.setOption(option);
  }

  // 按优先级统计违约
  async function initSLAViolationPriorityChart(statsData) {
    const chartDom = document.getElementById('sla-violation-priority-chart');
    if (!chartDom) return;

    if (slaCharts.violationPriority) {
      slaCharts.violationPriority.dispose();
    }

    slaCharts.violationPriority = echarts.init(chartDom);

    const violationsByPriority = statsData.violations_by_priority || {};

    const priorities = ['low', 'normal', 'high', 'urgent'];
    const priorityNames = { low: '低', normal: '正常', high: '高', urgent: '紧急' };
    const data = priorities.map(priority => violationsByPriority[priority] || 0);
    const names = priorities.map(priority => priorityNames[priority]);

    // 如果没有数据，使用模拟数据
    const hasData = data.some(value => value > 0);
    if (!hasData) {
      data[0] = 5;  // low
      data[1] = 8;  // normal
      data[2] = 7;  // high
      data[3] = 3;  // urgent
    }

    const option = {
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'shadow'
        }
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        data: names
      },
      yAxis: {
        type: 'value',
        name: '违约数量'
      },
      series: [
        {
          name: '违约数量',
          type: 'bar',
          data: data,
          itemStyle: {
            color: function(params) {
              const colors = ['#48bb78', '#4299e1', '#ecc94b', '#f56565'];
              return colors[params.dataIndex];
            }
          },
          emphasis: {
            itemStyle: {
              shadowBlur: 10,
              shadowOffsetX: 0,
              shadowColor: 'rgba(0, 0, 0, 0.5)'
            }
          }
        }
      ]
    };

    slaCharts.violationPriority.setOption(option);
  }

  // 加载SLA违约列表
  async function loadSLAViolations(page = 1, filters = {}) {
    try {
      const params = new URLSearchParams({
        page: page.toString(),
        page_size: slaPageSize.toString(),
        ...filters
      });

      const res = await jget(`${API}/sla/violations?${params}`);
      const tbody = $('#tbl_sla_violations tbody');
      tbody.innerHTML = '';

      if (res.data && res.data.length > 0) {
        res.data.forEach(violation => {
          const tr = document.createElement('tr');

          const typeBadge = `<span class="violation-type-badge violation-${violation.violation_type}">${violation.violation_type === 'first_response' ? '首次响应' : '解决时间'}</span>`;
          const resolvedBadge = violation.resolved ? '<span class="resolved-yes">已解决</span>' : '<span class="resolved-no">未解决</span>';
          const deadlineTime = new Date(violation.deadline).toLocaleString('zh-CN');
          const violatedTime = new Date(violation.violated_at).toLocaleString('zh-CN');

          tr.innerHTML = `
            <td>${violation.id}</td>
            <td>#${violation.ticket_id}</td>
            <td>${violation.sla_config?.name || '-'}</td>
            <td>${typeBadge}</td>
            <td>${deadlineTime}</td>
            <td>${violatedTime}</td>
            <td>${resolvedBadge}</td>
            <td>
              ${!violation.resolved ? `<button onclick="resolveSLAViolation(${violation.id})" style="background: #48bb78; color: white; padding: 4px 8px; font-size: 12px; border: none; border-radius: 4px; cursor: pointer;">标记解决</button>` : '-'}
            </td>
          `;
          tbody.appendChild(tr);
        });
      } else {
        tbody.innerHTML = '<tr><td colspan="8" style="text-align: center; color: #718096;">暂无SLA违约数据</td></tr>';
      }

      // 更新分页信息
      const totalPages = Math.ceil(res.total / slaPageSize);
      $('#sla_violations_page_info').textContent = `第${page}页 / 共${totalPages}页 (${res.total}条记录)`;
      $('#btn_sla_violations_prev').disabled = page <= 1;
      $('#btn_sla_violations_next').disabled = page >= totalPages;

      slaCurrentViolationPage = page;
    } catch (e) {
      $('#tbl_sla_violations tbody').innerHTML = `<tr><td colspan="8" style="text-align: center; color: #e53e3e;">加载失败: ${e.message}</td></tr>`;
    }
  }

  // 显示SLA配置模态框
  function showSLAConfigModal(isEdit = false, configData = null) {
    const modal = $('#sla_config_modal');
    const title = $('#sla_config_modal_title');
    const form = $('#form_sla_config');

    if (isEdit && configData) {
      title.textContent = '编辑SLA配置';
      $('#sla_config_name').value = configData.name || '';
      $('#sla_config_priority').value = configData.priority || '';
      $('#sla_config_first_response').value = configData.first_response_time || '';
      $('#sla_config_resolution').value = configData.resolution_time || '';
      $('#sla_config_escalation').value = configData.escalation_time || '';
      $('#sla_config_business_hours').checked = configData.business_hours_only || false;
      $('#sla_config_active').checked = configData.active !== false;
      slaConfigEditId = configData.id;
    } else {
      title.textContent = '新建SLA配置';
      form.reset();
      $('#sla_config_active').checked = true;
      slaConfigEditId = null;
    }

    modal.style.display = 'block';
  }

  // 隐藏SLA配置模态框
  function hideSLAConfigModal() {
    $('#sla_config_modal').style.display = 'none';
    slaConfigEditId = null;
  }

  // 保存SLA配置
  async function saveSLAConfig(formData) {
    try {
      const data = {
        name: formData.get('name'),
        priority: formData.get('priority'),
        first_response_time: parseInt(formData.get('first_response_time')),
        resolution_time: parseInt(formData.get('resolution_time')),
        escalation_time: parseInt(formData.get('escalation_time')),
        business_hours_only: formData.has('business_hours_only'),
        active: formData.has('active')
      };

      if (slaConfigEditId) {
        // 更新配置
        await jpost(`${API}/sla/configs/${slaConfigEditId}`, data);
        alert('SLA配置更新成功');
      } else {
        // 创建配置
        await jpost(`${API}/sla/configs`, data);
        alert('SLA配置创建成功');
      }

      hideSLAConfigModal();
      loadSLAConfigs(slaCurrentConfigPage);
      loadSLAStats();
    } catch (e) {
      alert('操作失败: ' + e.message);
    }
  }

  // 全局函数
  window.editSLAConfig = async function(id) {
    try {
      const config = await jget(`${API}/sla/configs/${id}`);
      showSLAConfigModal(true, config);
    } catch (e) {
      alert('获取配置失败: ' + e.message);
    }
  };

  window.deleteSLAConfig = async function(id) {
    if (!confirm('确定要删除这个SLA配置吗？此操作不可恢复。')) {
      return;
    }

    try {
      await fetch(`${API}/sla/configs/${id}`, { method: 'DELETE' });
      alert('删除成功');
      loadSLAConfigs(slaCurrentConfigPage);
      loadSLAStats();
    } catch (e) {
      alert('删除失败: ' + e.message);
    }
  };

  window.resolveSLAViolation = async function(id) {
    if (!confirm('确定要标记这个违约为已解决吗？')) {
      return;
    }

    try {
      await jpost(`${API}/sla/violations/${id}/resolve`);
      alert('标记成功');
      loadSLAViolations(slaCurrentViolationPage);
      loadSLAStats();
    } catch (e) {
      alert('操作失败: ' + e.message);
    }
  };

  // 事件监听器
  $('#btn_new_sla_config')?.addEventListener('click', () => {
    showSLAConfigModal(false);
  });

  $('#btn_sla_stats')?.addEventListener('click', async () => {
    try {
      await loadSLAStats();
      alert('统计数据已刷新');
    } catch (e) {
      alert('刷新统计失败: ' + e.message);
    }
  });

  $('.modal-close')?.addEventListener('click', hideSLAConfigModal);
  $('#btn_sla_config_cancel')?.addEventListener('click', hideSLAConfigModal);

  $('#sla_config_modal')?.addEventListener('click', (e) => {
    if (e.target.id === 'sla_config_modal') {
      hideSLAConfigModal();
    }
  });

  $('#form_sla_config')?.addEventListener('submit', (e) => {
    e.preventDefault();
    const formData = new FormData(e.target);
    saveSLAConfig(formData);
  });

  // SLA配置分页
  $('#btn_sla_configs_prev')?.addEventListener('click', () => {
    if (slaCurrentConfigPage > 1) {
      loadSLAConfigs(slaCurrentConfigPage - 1);
    }
  });

  $('#btn_sla_configs_next')?.addEventListener('click', () => {
    loadSLAConfigs(slaCurrentConfigPage + 1);
  });

  // SLA违约分页
  $('#btn_sla_violations_prev')?.addEventListener('click', () => {
    if (slaCurrentViolationPage > 1) {
      loadSLAViolations(slaCurrentViolationPage - 1);
    }
  });

  $('#btn_sla_violations_next')?.addEventListener('click', () => {
    loadSLAViolations(slaCurrentViolationPage + 1);
  });

  // SLA违约筛选
  $('#btn_sla_violation_filter')?.addEventListener('click', () => {
    const filters = {};

    const type = $('#sla_violation_filter_type').value;
    if (type) {
      filters.violation_type = type;
    }

    const resolved = $('#sla_violation_filter_resolved').value;
    if (resolved) {
      filters.resolved = resolved;
    }

    const dateFrom = $('#sla_violation_filter_date_from').value;
    if (dateFrom) {
      filters.date_from = dateFrom;
    }

    const dateTo = $('#sla_violation_filter_date_to').value;
    if (dateTo) {
      filters.date_to = dateTo;
    }

    loadSLAViolations(1, filters);
  });

  // 更新导航切换逻辑，包含SLA
  const originalSetActive = setActive;
  window.setActive = function(tab) {
    originalSetActive(tab);
    if (tab === 'satisfaction') {
      loadSatisfactions();
      loadSatisfactionStats();
    } else if (tab === 'dashboard') {
      setTimeout(() => {
        initializeDashboardCharts();
      }, 100);
    } else if (tab === 'tickets') {
      setTimeout(() => {
        initializeTicketCharts();
      }, 100);
    } else if (tab === 'sla') {
      // 加载SLA数据
      loadSLAConfigs();
      loadSLAViolations();
      loadSLAStats();
      // 稍后初始化图表
      setTimeout(() => {
        if (slaCharts.complianceTrend) {
          slaCharts.complianceTrend.resize();
        }
        if (slaCharts.violationType) {
          slaCharts.violationType.resize();
        }
        if (slaCharts.violationPriority) {
          slaCharts.violationPriority.resize();
        }
      }, 100);
    }
  };
  setActive = window.setActive;

  // 窗口大小变化时重新调整SLA图表大小
  const originalResize = window.addEventListener;
  window.addEventListener('resize', () => {
    clearTimeout(window.resizeTimeout);
    window.resizeTimeout = setTimeout(() => {
      // 调整仪表板图表大小
      Object.values(dashboardCharts).forEach(chart => {
        if (chart && typeof chart.resize === 'function') {
          chart.resize();
        }
      });

      // 调整满意度图表大小
      Object.values(satisfactionCharts).forEach(chart => {
        if (chart && typeof chart.resize === 'function') {
          chart.resize();
        }
      });

      // 调整工单图表大小
      Object.values(ticketCharts).forEach(chart => {
        if (chart && typeof chart.resize === 'function') {
          chart.resize();
        }
      });

      // 调整SLA图表大小
      Object.values(slaCharts).forEach(chart => {
        if (chart && typeof chart.resize === 'function') {
          chart.resize();
        }
      });
    }, 300);
  });
})();
