# Servify TODO

面向“能跑 + 可验收 + 可持续迭代”的待办清单（工程优先，其次产品能力对标）。

## 当前进度（快速结论）
- Go workspace：`go.work` 仅包含 `./apps/server`（根目录不能直接 `go test ./...`）
- 单测：`go -C apps/server test ./...` 通过
- 测试脚本：`./scripts/run-tests.sh` 通过（覆盖率阈值 >= 20%）
- 代码内未发现额外 TODO/FIXME 标记（目前只在本文件中）

## P0（立刻做，保证 CI/本地体验）
- [x] 提升覆盖率到 >= 20%，让 `./scripts/run-tests.sh` 通过
  - 验收：脚本末尾显示 `✅ Coverage target achieved!`
  - 建议：补齐 `apps/server/internal/handlers` 中核心管理 API（tickets/customers/agents/...）的 httptest 覆盖
- [x] 统一/澄清测试入口
  - 验收：README 中明确 `go test` 的正确方式（例如 `go -C apps/server test ./...`）并避免误导
  - 进度：README 已修正贡献指南中的测试命令

## P1（功能完善：后台管理/运营能力）
- [x] Admin Dashboard：图表可视化（趋势、分布）与导出（CSV）
  - 验收：Dashboard 页 4 个图表正常渲染，且支持 CSV 导出（工单趋势/满意度分布/客服负载/平台分布）
- [x] 工单：批量操作（状态/标签/指派）+ 工单转移（session/ticket transfer 的端到端流程）
  - 进度：`/api/tickets/bulk` + Tickets 页批量修改 UI 已完成；会话转接已接入 WebSocket“转人工”触发 + 管理后台会话转接页（等待队列/取消/历史/手动转接）
- [x] 权限：细粒度 RBAC（admin/agent 之外的资源级权限）
- [x] 自定义字段/动态表单：字段配置（校验、条件展示）与查询/导出联动

## P2（产品能力对标：Competitive Roadmap）
参考成熟客服产品（Zendesk、Freshdesk、ServiceNow）的能力差距清单（需评估数据模型、API 与前端交互后排期）。

### Zendesk Support Suite（已覆盖/已落地）
- [x] 全渠道代理工作台（WebSocket/平台路由/会话上下文）
- [x] SLA 策略引擎（配置/违约检测与处理）
- [x] 触发器 & 自动化（基于事件的规则）
- [x] 宏与回复模板
- [x] CSAT 调查流程
- [x] 应用市场集成点

### Freshdesk（待补齐）
- [x] 自助服务门户 / 知识库（MVP：`/public/kb/docs` + `kb.html` 公共页；`/api/knowledge-docs` 管理 CRUD）
- [x] AI 建议（MVP：`/api/assist/suggest` 意图识别 + 相似工单 + 知识库推荐）
- [ ] 动态表单与自定义字段
- [x] 工作流自动化（MVP：触发器 + `POST /api/automations/run` 批量执行 + `GET /api/automations/runs` 审计）
- [x] 多语言 & 品牌门户（MVP：`/public/portal/config` + `kb.html`/`satisfaction.html` 支持 `?lang=zh-CN|en-US` + 动态品牌色）
- [x] 绩效游戏化（MVP：`GET /api/gamification/leaderboard` + 徽章：解决王/满意之星/极速响应）

### ServiceNow（待补齐）
- [ ] Incident/Problem/Change（ITIL 流程联动）
- [ ] CMDB 关联
- [ ] Virtual Agent（可配置对话机器人）
- [ ] Performance Analytics（可配置 KPI + 告警）
- [ ] 服务目录 & 请求履约
- [ ] 审批与合规审计轨迹
