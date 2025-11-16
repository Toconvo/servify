import { ServifySDK, ServifyConfig, Message, Customer, ChatSession, Agent } from '@servify/core';

/**
 * 为原生 JavaScript 提供更简单的 API 接口
 */
export class VanillaServifySDK {
  private sdk: ServifySDK;
  private eventCallbacks: Map<string, Function[]> = new Map();

  constructor(config: ServifyConfig) {
    this.sdk = new ServifySDK(config);

    // 转发所有事件到回调函数
    this.sdk.on('connected', () => this.triggerCallback('connected'));
    this.sdk.on('disconnected', (reason) => this.triggerCallback('disconnected', reason));
    this.sdk.on('message', (message) => this.triggerCallback('message', message));
    this.sdk.on('session_created', (session) => this.triggerCallback('sessionCreated', session));
    this.sdk.on('session_updated', (session) => this.triggerCallback('sessionUpdated', session));
    this.sdk.on('session_ended', (session) => this.triggerCallback('sessionEnded', session));
    this.sdk.on('agent_assigned', (agent) => this.triggerCallback('agentAssigned', agent));
    this.sdk.on('agent_typing', (isTyping) => this.triggerCallback('agentTyping', isTyping));
    this.sdk.on('error', (error) => this.triggerCallback('error', error));
    this.sdk.on('ticket_created', (ticket) => this.triggerCallback('ticketCreated', ticket));
  }

  /**
   * 初始化 SDK
   */
  async init(): Promise<void> {
    return this.sdk.initialize();
  }

  /**
   * 连接到服务器
   */
  async connect(): Promise<void> {
    return this.sdk.connect();
  }

  /**
   * 断开连接
   */
  disconnect(): void {
    this.sdk.disconnect();
  }

  /**
   * 开始聊天
   */
  async startChat(options?: {
    priority?: string;
    message?: string;
  }): Promise<ChatSession> {
    return this.sdk.startChat({
      priority: options?.priority as any,
      message: options?.message,
    });
  }

  /**
   * 发送消息
   */
  async sendMessage(content: string, type: string = 'text'): Promise<Message> {
    return this.sdk.sendMessage(content, { type: type as any });
  }

  /**
   * 结束会话
   */
  async endChat(): Promise<void> {
    return this.sdk.endSession();
  }

  /**
   * AI 问答
   */
  async askAI(question: string): Promise<{ answer: string; confidence: number }> {
    return this.sdk.askAI(question);
  }

  /**
   * 上传文件
   */
  async uploadFile(file: File): Promise<{ fileUrl: string; fileName: string; fileSize: number }> {
    const result = await this.sdk.uploadFile(file);
    return {
      fileUrl: result.file_url,
      fileName: result.file_name,
      fileSize: result.file_size,
    };
  }

  /**
   * 创建工单
   */
  async createTicket(data: {
    title: string;
    description: string;
    priority?: string;
    category: string;
  }): Promise<any> {
    return this.sdk.createTicket({
      ...data,
      priority: data.priority as any,
    });
  }

  /**
   * 提交满意度评价
   */
  async submitRating(rating: number, comment?: string): Promise<any> {
    return this.sdk.submitSatisfaction({
      rating,
      comment,
    });
  }

  /**
   * 获取历史消息
   */
  async getMessages(page: number = 1, limit: number = 50): Promise<{
    messages: Message[];
    total: number;
    page: number;
  }> {
    return this.sdk.getMessages({ page, limit });
  }

  /**
   * 获取客户信息
   */
  getCustomer(): Customer | null {
    return this.sdk.getCustomer();
  }

  /**
   * 获取当前会话
   */
  getSession(): ChatSession | null {
    return this.sdk.getSession();
  }

  /**
   * 获取当前客服代理
   */
  getAgent(): Agent | null {
    return this.sdk.getAgent();
  }

  /**
   * 检查连接状态
   */
  isConnected(): boolean {
    return this.sdk.isConnected();
  }

  /**
   * 添加事件监听器（简化版）
   */
  on(event: string, callback: Function): void {
    if (!this.eventCallbacks.has(event)) {
      this.eventCallbacks.set(event, []);
    }
    this.eventCallbacks.get(event)!.push(callback);
  }

  /**
   * 移除事件监听器
   */
  off(event: string, callback?: Function): void {
    if (!callback) {
      this.eventCallbacks.delete(event);
      return;
    }

    const callbacks = this.eventCallbacks.get(event);
    if (callbacks) {
      const index = callbacks.indexOf(callback);
      if (index > -1) {
        callbacks.splice(index, 1);
      }
    }
  }

  /**
   * 触发回调函数
   */
  private triggerCallback(event: string, ...args: any[]): void {
    const callbacks = this.eventCallbacks.get(event);
    if (callbacks) {
      callbacks.forEach(callback => {
        try {
          callback(...args);
        } catch (error) {
          console.error(`Error in ${event} callback:`, error);
        }
      });
    }
  }
}

// 全局变量注册（用于浏览器环境）
declare global {
  interface Window {
    Servify: typeof VanillaServifySDK;
    createServify: (config: ServifyConfig) => VanillaServifySDK;
  }
}

// 浏览器环境下的全局注册
if (typeof window !== 'undefined') {
  window.Servify = VanillaServifySDK;
  window.createServify = (config: ServifyConfig) => new VanillaServifySDK(config);
}

export { ServifyConfig, Message, Customer, ChatSession, Agent } from '@servify/core';
export default VanillaServifySDK;