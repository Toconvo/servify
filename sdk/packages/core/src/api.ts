import { ApiResponse, Customer, Agent, ChatSession, Message, Ticket, CustomerSatisfaction } from './types';

export interface ApiClientOptions {
  baseUrl: string;
  timeout?: number;
  headers?: Record<string, string>;
  debug?: boolean;
}

export class ApiClient {
  private options: Required<ApiClientOptions>;

  constructor(options: ApiClientOptions) {
    this.options = {
      timeout: 10000,
      headers: {
        'Content-Type': 'application/json',
      },
      debug: false,
      ...options
    };
  }

  // 通用请求方法
  private async request<T>(
    method: 'GET' | 'POST' | 'PUT' | 'DELETE',
    endpoint: string,
    data?: any,
    options?: Partial<ApiClientOptions>
  ): Promise<ApiResponse<T>> {
    const url = `${this.options.baseUrl}${endpoint}`;
    const headers = { ...this.options.headers, ...options?.headers };

    this.log(`${method} ${url}`, data);

    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), this.options.timeout);

      const response = await fetch(url, {
        method,
        headers,
        body: data ? JSON.stringify(data) : undefined,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      const result = await response.json();
      this.log(`响应:`, result);

      if (!response.ok) {
        return {
          success: false,
          error: result.error || `HTTP ${response.status}: ${response.statusText}`,
        };
      }

      return {
        success: true,
        data: result.data || result,
        message: result.message,
      };
    } catch (error) {
      this.log(`请求失败:`, error);

      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          return { success: false, error: 'Request timeout' };
        }
        return { success: false, error: error.message };
      }

      return { success: false, error: 'Unknown error' };
    }
  }

  // 客户相关 API
  async getCustomer(customerId: number): Promise<ApiResponse<Customer>> {
    return this.request<Customer>('GET', `/api/customers/${customerId}`);
  }

  async createCustomer(customerData: Partial<Customer>): Promise<ApiResponse<Customer>> {
    return this.request<Customer>('POST', '/api/customers', customerData);
  }

  async updateCustomer(customerId: number, customerData: Partial<Customer>): Promise<ApiResponse<Customer>> {
    return this.request<Customer>('PUT', `/api/customers/${customerId}`, customerData);
  }

  // 会话相关 API
  async createSession(sessionData: Partial<ChatSession>): Promise<ApiResponse<ChatSession>> {
    return this.request<ChatSession>('POST', '/api/sessions', sessionData);
  }

  async getSession(sessionId: number): Promise<ApiResponse<ChatSession>> {
    return this.request<ChatSession>('GET', `/api/sessions/${sessionId}`);
  }

  async endSession(sessionId: number): Promise<ApiResponse<void>> {
    return this.request<void>('PUT', `/api/sessions/${sessionId}/end`);
  }

  async getCustomerSessions(customerId: number): Promise<ApiResponse<ChatSession[]>> {
    return this.request<ChatSession[]>('GET', `/api/customers/${customerId}/sessions`);
  }

  // 消息相关 API
  async sendMessage(messageData: Partial<Message>): Promise<ApiResponse<Message>> {
    return this.request<Message>('POST', '/api/messages', messageData);
  }

  async getSessionMessages(sessionId: number, options?: {
    page?: number;
    limit?: number;
  }): Promise<ApiResponse<{ messages: Message[]; total: number; page: number }>> {
    const params = new URLSearchParams();
    if (options?.page) params.append('page', options.page.toString());
    if (options?.limit) params.append('limit', options.limit.toString());

    const query = params.toString() ? `?${params.toString()}` : '';
    return this.request('GET', `/api/sessions/${sessionId}/messages${query}`);
  }

  // AI 相关 API
  async askAI(question: string, sessionId?: number): Promise<ApiResponse<{ answer: string; confidence: number }>> {
    return this.request('POST', '/api/ai/ask', { question, session_id: sessionId });
  }

  async getAIStatus(): Promise<ApiResponse<{ status: string; models: string[] }>> {
    return this.request('GET', '/api/ai/status');
  }

  // 工单相关 API
  async createTicket(ticketData: Partial<Ticket>): Promise<ApiResponse<Ticket>> {
    return this.request<Ticket>('POST', '/api/tickets', ticketData);
  }

  async getTicket(ticketId: number): Promise<ApiResponse<Ticket>> {
    return this.request<Ticket>('GET', `/api/tickets/${ticketId}`);
  }

  async updateTicket(ticketId: number, updates: Partial<Ticket>): Promise<ApiResponse<Ticket>> {
    return this.request<Ticket>('PUT', `/api/tickets/${ticketId}`, updates);
  }

  async getCustomerTickets(customerId: number): Promise<ApiResponse<Ticket[]>> {
    return this.request<Ticket[]>('GET', `/api/customers/${customerId}/tickets`);
  }

  // 满意度评价 API
  async submitSatisfaction(satisfactionData: Partial<CustomerSatisfaction>): Promise<ApiResponse<CustomerSatisfaction>> {
    return this.request<CustomerSatisfaction>('POST', '/api/satisfaction', satisfactionData);
  }

  async getSatisfactionByTicket(ticketId: number): Promise<ApiResponse<CustomerSatisfaction>> {
    return this.request<CustomerSatisfaction>('GET', `/api/tickets/${ticketId}/satisfaction`);
  }

  // 队列相关 API
  async joinQueue(queueData: {
    customer_id: number;
    priority?: string;
    estimated_wait?: number;
  }): Promise<ApiResponse<{ queue_position: number; estimated_wait: number }>> {
    return this.request('POST', '/api/queue/join', queueData);
  }

  async getQueueStatus(customerId: number): Promise<ApiResponse<{
    in_queue: boolean;
    position?: number;
    estimated_wait?: number;
  }>> {
    return this.request('GET', `/api/queue/status/${customerId}`);
  }

  async leaveQueue(customerId: number): Promise<ApiResponse<void>> {
    return this.request('DELETE', `/api/queue/leave/${customerId}`);
  }

  // 文件上传 API
  async uploadFile(file: File, sessionId: number): Promise<ApiResponse<{
    file_url: string;
    file_name: string;
    file_size: number;
  }>> {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('session_id', sessionId.toString());

    const url = `${this.options.baseUrl}/api/upload`;

    try {
      const response = await fetch(url, {
        method: 'POST',
        body: formData,
        headers: {
          // 不设置 Content-Type，让浏览器自动设置 multipart/form-data 边界
          ...Object.fromEntries(
            Object.entries(this.options.headers).filter(([key]) =>
              key.toLowerCase() !== 'content-type'
            )
          )
        }
      });

      const result = await response.json();

      if (!response.ok) {
        return { success: false, error: result.error || 'Upload failed' };
      }

      return { success: true, data: result.data || result };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Upload failed'
      };
    }
  }

  // WebRTC 相关 API
  async startCall(sessionId: number, callType: 'audio' | 'video'): Promise<ApiResponse<{
    call_id: number;
    ice_servers: RTCIceServer[];
  }>> {
    return this.request('POST', `/api/webrtc/call/start`, { session_id: sessionId, type: callType });
  }

  async endCall(callId: number): Promise<ApiResponse<void>> {
    return this.request('PUT', `/api/webrtc/call/${callId}/end`);
  }

  async getCallStatus(callId: number): Promise<ApiResponse<{
    status: string;
    duration?: number;
  }>> {
    return this.request('GET', `/api/webrtc/call/${callId}/status`);
  }

  // 设置认证头
  setAuthToken(token: string): void {
    this.options.headers['Authorization'] = `Bearer ${token}`;
  }

  // 设置客户 ID
  setCustomerId(customerId: number): void {
    this.options.headers['X-Customer-ID'] = customerId.toString();
  }

  private log(...args: any[]): void {
    if (this.options.debug) {
      console.log('[ServifyAPI]', ...args);
    }
  }
}