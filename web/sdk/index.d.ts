// Servify Web SDK Type Definitions
// Basic .d.ts for ESM/UMD consumers in TS projects (React/Vue/Angular)

export type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

export interface ICEConfig {
  urls: string[];
}

export interface ServifyConfig {
  wsUrl?: string;
  baseUrl?: string;
  sessionId?: string;
  autoConnect?: boolean;
  autoReconnect?: boolean;
  reconnectAttempts?: number;
  reconnectDelayMs?: number;
  reconnectDelayMaxMs?: number;
  heartbeatIntervalMs?: number;
  stunServers?: ICEConfig[];
}

export interface AIResponse {
  content: string;
  confidence?: number;
  source?: string;
}

export type EventMap = {
  open: () => void;
  close: (ev?: CloseEvent) => void;
  error: (err: Event | Error) => void;
  status: (status: ConnectionStatus) => void;
  message: (payload: any) => void;
  ai: (data: AIResponse) => void;
  'webrtc:state': (state: RTCPeerConnectionState | 'closed') => void;
  'webrtc:offer': (offer: RTCSessionDescriptionInit) => void;
  'webrtc:answer': (answer: RTCSessionDescriptionInit) => void;
  'webrtc:candidate': (cand: RTCIceCandidateInit) => void;
};

export class ServifyClient {
  constructor(config?: ServifyConfig);
  connect(): Promise<void>;
  disconnect(code?: number, reason?: string): void;
  sendRaw(payload: any): void;
  sendMessage(text: string): void;
  getStatus(): ConnectionStatus;

  // Events
  on<K extends keyof EventMap>(type: K, handler: EventMap[K]): () => void;
  off<K extends keyof EventMap>(type: K, handler: EventMap[K]): void;

  // WebRTC
  startRemoteAssist(constraints?: { video?: boolean; audio?: boolean; preferDisplay?: boolean }): Promise<RTCPeerConnection>;
  acceptRemoteAnswer(answer: RTCSessionDescriptionInit): Promise<void>;
  addRemoteIce(candidate: RTCIceCandidateInit): Promise<void>;
  endRemoteAssist(): void;
}

export function createClient(config?: ServifyConfig): ServifyClient;

export default ServifyClient;
