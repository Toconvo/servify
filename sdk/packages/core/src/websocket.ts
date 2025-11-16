import EventEmitter from 'eventemitter3';
import { WSMessage, ServifyEventMap } from './types';

export interface WebSocketManagerOptions {
  url: string;
  protocols?: string | string[];
  reconnectAttempts?: number;
  reconnectDelay?: number;
  heartbeatInterval?: number;
  debug?: boolean;
}

export class WebSocketManager extends EventEmitter<ServifyEventMap> {
  private ws: WebSocket | null = null;
  private options: Required<WebSocketManagerOptions>;
  private reconnectAttempts = 0;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private heartbeatTimer: NodeJS.Timeout | null = null;
  private isManualClose = false;

  constructor(options: WebSocketManagerOptions) {
    super();

    this.options = {
      protocols: [],
      reconnectAttempts: 5,
      reconnectDelay: 1000,
      heartbeatInterval: 30000,
      debug: false,
      ...options
    };
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        resolve();
        return;
      }

      this.isManualClose = false;
      this.log('正在连接 WebSocket...', this.options.url);

      try {
        this.ws = new WebSocket(this.options.url, this.options.protocols);
      } catch (error) {
        reject(error);
        return;
      }

      this.ws.onopen = () => {
        this.log('WebSocket 连接成功');
        this.reconnectAttempts = 0;
        this.startHeartbeat();
        this.emit('connected');
        resolve();
      };

      this.ws.onmessage = (event) => {
        try {
          const message: WSMessage = JSON.parse(event.data);
          this.handleMessage(message);
        } catch (error) {
          this.log('解析消息失败:', error);
          this.emit('error', new Error('Invalid message format'));
        }
      };

      this.ws.onclose = (event) => {
        this.log('WebSocket 连接关闭:', event.code, event.reason);
        this.stopHeartbeat();
        this.emit('disconnected', event.reason || '连接关闭');

        if (!this.isManualClose && this.reconnectAttempts < this.options.reconnectAttempts) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = (event) => {
        this.log('WebSocket 错误:', event);
        this.emit('error', new Error('WebSocket connection error'));
        reject(new Error('WebSocket connection error'));
      };
    });
  }

  disconnect(): void {
    this.isManualClose = true;
    this.stopHeartbeat();

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  send(message: WSMessage): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      this.log('WebSocket 未连接，无法发送消息');
      return false;
    }

    try {
      this.ws.send(JSON.stringify(message));
      this.log('发送消息:', message);
      return true;
    } catch (error) {
      this.log('发送消息失败:', error);
      this.emit('error', new Error('Failed to send message'));
      return false;
    }
  }

  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  private handleMessage(message: WSMessage): void {
    this.log('收到消息:', message);

    switch (message.type) {
      case 'message':
        this.emit('message', message.data);
        break;
      case 'session_update':
        this.emit('session_updated', message.data);
        break;
      case 'agent_status':
        if (message.data.type === 'assigned') {
          this.emit('agent_assigned', message.data.agent);
        } else if (message.data.type === 'typing') {
          this.emit('agent_typing', message.data.typing);
        }
        break;
      case 'error':
        this.emit('error', new Error(message.data.message || 'Unknown error'));
        break;
      case 'system':
        // 处理系统消息，如心跳响应
        if (message.data?.type === 'pong') {
          // 心跳响应处理
          this.log('收到心跳响应');
        }
        break;
      default:
        this.log('未知消息类型:', message.type);
    }
  }

  private scheduleReconnect(): void {
    this.reconnectAttempts++;
    this.emit('reconnecting', this.reconnectAttempts);

    const delay = this.options.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
    this.log(`${delay}ms 后重连 (第 ${this.reconnectAttempts}/${this.options.reconnectAttempts} 次)`);

    this.reconnectTimer = setTimeout(() => {
      this.connect().catch(() => {
        // 重连失败，继续尝试或放弃
      });
    }, delay);
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();

    this.heartbeatTimer = setInterval(() => {
      if (this.isConnected()) {
        this.send({
          type: 'system',
          data: { type: 'ping', timestamp: new Date().toISOString() }
        });
      }
    }, this.options.heartbeatInterval);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private log(...args: any[]): void {
    if (this.options.debug) {
      console.log('[ServifyWS]', ...args);
    }
  }
}