import EventEmitter from 'eventemitter3';
import { ApiClient } from './api';
import { WebSocketManager } from './websocket';
import {
  ServifyConfig,
  ServifyEventMap,
  Customer,
  Agent,
  ChatSession,
  Message,
  Ticket,
  CustomerSatisfaction,
  WebRTCConfig,
  WebRTCCall
} from './types';

export class ServifySDK extends EventEmitter<ServifyEventMap> {
  private config: ServifyConfig;
  private api: ApiClient;
  private ws: WebSocketManager | null = null;
  private currentCustomer: Customer | null = null;
  private currentSession: ChatSession | null = null;
  private currentAgent: Agent | null = null;
  private messageQueue: Message[] = [];
  private isInitialized = false;

  constructor(config: ServifyConfig) {
    super();

    this.config = {
      autoConnect: true,
      reconnectAttempts: 5,
      reconnectDelay: 1000,
      debug: false,
      ...config
    };

    // 初始化 API 客户端
    this.api = new ApiClient({
      baseUrl: this.config.apiUrl,
      debug: this.config.debug,
    });

    // 如果提供了客户信息，设置到 API 客户端
    if (this.config.customerId) {
      this.api.setCustomerId(parseInt(this.config.customerId));
    }

    this.log('SDK 初始化完成', this.config);
  }

  // 初始化 SDK
  async initialize(): Promise<void> {
    if (this.isInitialized) {
      this.log('SDK 已初始化');
      return;
    }

    try {
      this.log('正在初始化 SDK...');

      // 获取或创建客户信息
      await this.initializeCustomer();

      // 初始化 WebSocket 连接
      if (this.config.autoConnect) {
        await this.connect();
      }

      this.isInitialized = true;
      this.log('SDK 初始化成功');

    } catch (error) {
      this.log('SDK 初始化失败:', error);
      throw error;
    }
  }

  // 连接到服务器
  async connect(): Promise<void> {
    if (!this.currentCustomer) {
      throw new Error('Customer not initialized. Call initialize() first.');
    }

    const wsUrl = this.config.wsUrl || this.config.apiUrl.replace(/^http/, 'ws') + '/ws';

    this.ws = new WebSocketManager({
      url: `${wsUrl}?customer_id=${this.currentCustomer.id}`,
      reconnectAttempts: this.config.reconnectAttempts!,
      reconnectDelay: this.config.reconnectDelay!,
      debug: this.config.debug,
    });

    // 转发 WebSocket 事件
    this.ws.on('connected', () => this.emit('connected'));
    this.ws.on('disconnected', (reason) => this.emit('disconnected', reason));
    this.ws.on('reconnecting', (attempt) => this.emit('reconnecting', attempt));
    this.ws.on('message', (message) => this.handleIncomingMessage(message));
    this.ws.on('session_updated', (session) => {
      this.currentSession = session;
      this.emit('session_updated', session);
    });
    this.ws.on('agent_assigned', (agent) => {
      this.currentAgent = agent;
      this.emit('agent_assigned', agent);
    });
    this.ws.on('agent_typing', (isTyping) => this.emit('agent_typing', isTyping));
    this.ws.on('error', (error) => this.emit('error', error));

    await this.ws.connect();
  }

  // 断开连接
  disconnect(): void {
    this.ws?.disconnect();
    this.ws = null;
  }

  // 开始聊天会话
  async startChat(options?: {
    priority?: 'low' | 'normal' | 'high' | 'urgent';
    message?: string;
    metadata?: Record<string, any>;
  }): Promise<ChatSession> {
    if (!this.currentCustomer) {
      throw new Error('Customer not initialized');
    }

    const sessionData: Partial<ChatSession> = {
      customer_id: this.currentCustomer.id,
      status: 'active',
      channel: 'web',
      priority: options?.priority || 'normal',
    };

    const response = await this.api.createSession(sessionData);
    if (!response.success || !response.data) {
      throw new Error(response.error || 'Failed to create session');
    }

    this.currentSession = response.data;
    this.emit('session_created', this.currentSession);

    // 如果有初始消息，发送它
    if (options?.message) {
      await this.sendMessage(options.message);
    }

    return this.currentSession;
  }

  // 发送消息
  async sendMessage(content: string, options?: {
    type?: 'text' | 'image' | 'file';
    attachments?: string[];
    metadata?: Record<string, any>;
  }): Promise<Message> {
    if (!this.currentSession) {
      throw new Error('No active session. Start a chat first.');
    }

    const messageData: Partial<Message> = {
      session_id: this.currentSession.id,
      sender_type: 'customer',
      sender_id: this.currentCustomer?.id,
      content,
      message_type: options?.type || 'text',
      attachments: options?.attachments,
      metadata: options?.metadata,
    };

    const response = await this.api.sendMessage(messageData);
    if (!response.success || !response.data) {
      throw new Error(response.error || 'Failed to send message');
    }

    // 通过 WebSocket 实时发送（如果连接可用）
    if (this.ws?.isConnected()) {
      this.ws.send({
        type: 'message',
        data: response.data,
      });
    }

    return response.data;
  }

  // 结束会话
  async endSession(): Promise<void> {
    if (!this.currentSession) {
      return;
    }

    const response = await this.api.endSession(this.currentSession.id);
    if (!response.success) {
      throw new Error(response.error || 'Failed to end session');
    }

    const endedSession = { ...this.currentSession, status: 'closed' as const };
    this.currentSession = null;
    this.currentAgent = null;
    this.emit('session_ended', endedSession);
  }

  // 创建工单
  async createTicket(ticketData: {
    title: string;
    description: string;
    priority?: 'low' | 'normal' | 'high' | 'urgent';
    category: string;
    metadata?: Record<string, any>;
  }): Promise<Ticket> {
    if (!this.currentCustomer) {
      throw new Error('Customer not initialized');
    }

    const data: Partial<Ticket> = {
      ...ticketData,
      customer_id: this.currentCustomer.id,
      status: 'open',
    };

    const response = await this.api.createTicket(data);
    if (!response.success || !response.data) {
      throw new Error(response.error || 'Failed to create ticket');
    }

    this.emit('ticket_created', response.data);
    return response.data;
  }

  // 提交满意度评价
  async submitSatisfaction(satisfaction: {
    ticket_id?: number;
    rating: number;
    comment?: string;
    category?: string;
  }): Promise<CustomerSatisfaction> {
    if (!this.currentCustomer) {
      throw new Error('Customer not initialized');
    }

    const data: Partial<CustomerSatisfaction> = {
      ...satisfaction,
      customer_id: this.currentCustomer.id,
      agent_id: this.currentAgent?.id,
    };

    const response = await this.api.submitSatisfaction(data);
    if (!response.success || !response.data) {
      throw new Error(response.error || 'Failed to submit satisfaction');
    }

    return response.data;
  }

  // AI 问答
  async askAI(question: string): Promise<{ answer: string; confidence: number }> {
    const response = await this.api.askAI(question, this.currentSession?.id);
    if (!response.success || !response.data) {
      throw new Error(response.error || 'Failed to get AI response');
    }

    return response.data;
  }

  // 文件上传
  async uploadFile(file: File): Promise<{ file_url: string; file_name: string; file_size: number }> {
    if (!this.currentSession) {
      throw new Error('No active session');
    }

    const response = await this.api.uploadFile(file, this.currentSession.id);
    if (!response.success || !response.data) {
      throw new Error(response.error || 'Failed to upload file');
    }

    return response.data;
  }

  // 获取历史消息
  async getMessages(options?: {
    page?: number;
    limit?: number;
  }): Promise<{ messages: Message[]; total: number; page: number }> {
    if (!this.currentSession) {
      throw new Error('No active session');
    }

    const response = await this.api.getSessionMessages(this.currentSession.id, options);
    if (!response.success || !response.data) {
      throw new Error(response.error || 'Failed to get messages');
    }

    return response.data;
  }

  // 获取客户信息
  getCustomer(): Customer | null {
    return this.currentCustomer;
  }

  // 获取当前会话
  getSession(): ChatSession | null {
    return this.currentSession;
  }

  // 获取当前客服代理
  getAgent(): Agent | null {
    return this.currentAgent;
  }

  // 检查连接状态
  isConnected(): boolean {
    return this.ws?.isConnected() ?? false;
  }

  // 私有方法：初始化客户信息
  private async initializeCustomer(): Promise<void> {
    if (this.config.customerId) {
      // 获取现有客户
      const response = await this.api.getCustomer(parseInt(this.config.customerId));
      if (response.success && response.data) {
        this.currentCustomer = response.data;
        return;
      }
    }

    // 创建新客户
    const customerData: Partial<Customer> = {
      name: this.config.customerName || 'Anonymous',
      email: this.config.customerEmail || '',
      status: 'active',
    };

    const response = await this.api.createCustomer(customerData);
    if (!response.success || !response.data) {
      throw new Error(response.error || 'Failed to create customer');
    }

    this.currentCustomer = response.data;
    this.api.setCustomerId(this.currentCustomer.id);
  }

  // 私有方法：处理收到的消息
  private handleIncomingMessage(message: Message): void {
    this.messageQueue.push(message);
    this.emit('message', message);
  }

  // 私有方法：日志输出
  private log(...args: any[]): void {
    if (this.config.debug) {
      console.log('[ServifySDK]', ...args);
    }
  }
}