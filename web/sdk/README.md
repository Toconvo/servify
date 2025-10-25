# Servify Web SDK

轻量的浏览器端 SDK，支持原生 Web、React、Vue、Angular 等框架集成。提供：
- WebSocket 文本聊天（`sendMessage` / `on('ai', ...)`）
- 基础事件（`open/close/error/status/message`）
- 可选 WebRTC 远程协助（屏幕共享 + 信令经 WebSocket）

文件：
- `servify-sdk.esm.js`：ES Module，适用于现代打包器（Vite/Webpack/Rollup）
- `servify-sdk.umd.js`：UMD，全局 `window.Servify`，可直接 `<script>` 引入
- `index.d.ts`：TypeScript 类型声明

## 浏览器直接使用（UMD）
```html
<script src="/sdk/servify-sdk.umd.js"></script>
<script>
  const client = Servify.createClient({ sessionId: 'demo_1' });
  client.on('open', () => console.log('connected'));
  client.on('ai', (res) => console.log('AI:', res.content));
  client.connect();
  // 发送文本
  client.sendMessage('你好');
</script>
```

## ES Module（React/Vue/Angular 打包器）
```ts
import { ServifyClient } from '/sdk/servify-sdk.esm.js';

const client = new ServifyClient({ baseUrl: 'http://localhost:8080' });
client.on('ai', (r) => console.log(r.content));
await client.connect();
client.sendMessage('help me');
```

- `wsUrl` 未指定时，SDK 会根据 `baseUrl` 或 `window.location` 生成 `ws(s)://.../api/v1/ws?session_id=...`

## React Hook 示例
```tsx
import { useEffect, useMemo, useState } from 'react';
import { ServifyClient } from '/sdk/servify-sdk.esm.js';

export function useServify(config) {
  const client = useMemo(() => new ServifyClient(config), [JSON.stringify(config)]);
  const [status, setStatus] = useState(client.getStatus());
  const [messages, setMessages] = useState([]);

  useEffect(() => {
    const offStatus = client.on('status', setStatus);
    const offAI = client.on('ai', (m) => setMessages((prev) => [...prev, { role: 'ai', ...m }]));
    const offMsg = client.on('message', (m) => {
      if (m?.type === 'text-message') setMessages((p) => [...p, { role: 'user', content: m.data?.content }]);
    });
    client.connect();
    return () => { offStatus(); offAI(); offMsg(); client.disconnect(); };
  }, [client]);

  return { client, status, messages };
}
```

## Vue 3 组合式示例
```ts
import { ref, onMounted, onBeforeUnmount } from 'vue';
import { ServifyClient } from '/sdk/servify-sdk.esm.js';

export function useServify(config) {
  const client = new ServifyClient(config);
  const status = ref(client.getStatus());
  const messages = ref([]);

  onMounted(() => {
    const offStatus = client.on('status', (s) => (status.value = s));
    const offAI = client.on('ai', (m) => messages.value.push({ role: 'ai', ...m }));
    const offMsg = client.on('message', (m) => { if (m?.type === 'text-message') messages.value.push({ role: 'user', content: m.data?.content }); });
    client.connect();
    onBeforeUnmount(() => { offStatus(); offAI(); offMsg(); client.disconnect(); });
  });

  return { client, status, messages };
}
```

## Angular Service 示例
```ts
// servify.service.ts
import { Injectable, NgZone } from '@angular/core';
import { BehaviorSubject, Subject } from 'rxjs';
import { ServifyClient } from '/sdk/servify-sdk.esm.js';

@Injectable({ providedIn: 'root' })
export class ServifyService {
  private client = new ServifyClient({ baseUrl: 'http://localhost:8080' });
  status$ = new BehaviorSubject(this.client.getStatus());
  ai$ = new Subject<any>();

  constructor(private zone: NgZone) {
    this.client.on('status', s => this.zone.run(() => this.status$.next(s)));
    this.client.on('ai', m => this.zone.run(() => this.ai$.next(m)));
    this.client.connect();
  }

  send(text: string) { this.client.sendMessage(text); }
}
```

## WebRTC 远程协助
> 当前服务端为会话内广播信令，实际对端需响应 `webrtc-answer` 才能建立连接。可在另一个页面/坐席端接收 `webrtc-offer` 后返回 `webrtc-answer`。

```ts
const pc = await client.startRemoteAssist(); // 发送 offer
client.on('webrtc:answer', (answer) => client.acceptRemoteAnswer(answer));
client.on('webrtc:candidate', (cand) => client.addRemoteIce(cand));
// 结束
client.endRemoteAssist();
```

## 事件一览
- `open/close/error`：WebSocket 连接生命周期
- `status`：`idle|connecting|connected|reconnecting|disconnected`
- `message`：所有原始消息（已 JSON 解析）
- `ai`：AI 回复（`{ content, confidence?, source? }`）
- `webrtc:*`：`state/offer/answer/candidate`

## 常见配置
```js
new ServifyClient({
  baseUrl: 'http://localhost:8080',
  sessionId: 'user_123',
  autoReconnect: true,
  reconnectDelayMs: 500,
  reconnectDelayMaxMs: 5000,
  stunServers: [{ urls: ['stun:stun1.l.google.com:19302'] }],
});
```

## 小挂件（Widget）
- 一行引入：
```html
<link rel="stylesheet" href="/sdk/widget.css" />
<script src="/sdk/servify-sdk.umd.js"></script>
<script src="/sdk/widget.js" data-servify-widget data-base-url="/" data-session-id="user_123"></script>
```
- 手动创建：
```html
<script>
  const w = ServifyWidget.create({ baseUrl: 'http://localhost:8080', sessionId: 'u_1' });
  // w.client 可直接调用 SDK API
  // w.mount 为挂件根节点
</script>
```

