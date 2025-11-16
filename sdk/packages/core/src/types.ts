// 基础类型定义
export interface ServifyConfig {
  apiUrl: string;
  wsUrl?: string;
  customerId?: string;
  customerName?: string;
  customerEmail?: string;
  sessionId?: string;
  debug?: boolean;
  autoConnect?: boolean;
  reconnectAttempts?: number;
  reconnectDelay?: number;
}

export interface Customer {
  id: number;
  name: string;
  email: string;
  phone?: string;
  address?: string;
  avatar?: string;
  status: 'active' | 'inactive';
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface Agent {
  id: number;
  name: string;
  email: string;
  avatar?: string;
  status: 'online' | 'offline' | 'busy' | 'away';
  is_ai?: boolean;
  created_at: string;
  updated_at: string;
}

export interface ChatSession {
  id: number;
  customer_id: number;
  agent_id?: number;
  status: 'active' | 'closed' | 'transferred';
  channel: 'web' | 'mobile' | 'email' | 'phone';
  priority: 'low' | 'normal' | 'high' | 'urgent';
  queue_id?: number;
  started_at: string;
  ended_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Message {
  id: number;
  session_id: number;
  sender_type: 'customer' | 'agent' | 'system';
  sender_id?: number;
  content: string;
  message_type: 'text' | 'image' | 'file' | 'system';
  attachments?: string[];
  is_ai_response?: boolean;
  metadata?: Record<string, any>;
  created_at: string;
}

export interface Ticket {
  id: number;
  customer_id: number;
  assigned_agent_id?: number;
  title: string;
  description: string;
  status: 'open' | 'in_progress' | 'resolved' | 'closed';
  priority: 'low' | 'normal' | 'high' | 'urgent';
  category: string;
  tags?: string[];
  metadata?: Record<string, any>;
  created_at: string;
  updated_at: string;
  resolved_at?: string;
}

export interface CustomerSatisfaction {
  id: number;
  ticket_id: number;
  customer_id: number;
  agent_id?: number;
  rating: number; // 1-5
  comment?: string;
  category?: string;
  created_at: string;
}

// WebSocket 消息类型
export interface WSMessage {
  type: 'message' | 'session_update' | 'agent_status' | 'typing' | 'error' | 'system';
  data: any;
  timestamp?: string;
}

// 事件类型
export type ServifyEventMap = {
  'connected': [];
  'disconnected': [reason: string];
  'reconnecting': [attempt: number];
  'message': [message: Message];
  'session_created': [session: ChatSession];
  'session_updated': [session: ChatSession];
  'session_ended': [session: ChatSession];
  'agent_assigned': [agent: Agent];
  'agent_typing': [isTyping: boolean];
  'error': [error: Error];
  'ticket_created': [ticket: Ticket];
  'ticket_updated': [ticket: Ticket];
};

// API 响应类型
export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  message?: string;
  error?: string;
}

// WebRTC 相关类型
export interface WebRTCConfig {
  iceServers?: RTCIceServer[];
  video?: boolean;
  audio?: boolean;
}

export interface WebRTCCall {
  id: number;
  session_id: number;
  caller_type: 'customer' | 'agent';
  status: 'ringing' | 'active' | 'ended';
  start_time?: string;
  end_time?: string;
  duration?: number;
}