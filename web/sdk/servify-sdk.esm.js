/*
 Servify Web SDK (ESM)
 Minimal, framework-agnostic browser SDK for chat (WebSocket) and optional WebRTC remote assist.
 - Works directly in modern browsers and with bundlers (React/Vue/Angular)
 - Exposes a small event API and simple methods
*/

const DEFAULTS = {
  autoConnect: true,
  autoReconnect: true,
  reconnectAttempts: Infinity,
  reconnectDelayMs: 500,
  reconnectDelayMaxMs: 5000,
  heartbeatIntervalMs: 25000,
  stunServers: [
    { urls: ['stun:stun.l.google.com:19302'] },
  ],
};

function backoff(attempt, base, max) {
  const jitter = Math.random() * 0.2 + 0.9; // 0.9x~1.1x
  const delay = Math.min(max, base * Math.pow(2, Math.max(0, attempt - 1)));
  return Math.floor(delay * jitter);
}

class Emitter {
  constructor() { this._events = new Map(); }
  on(type, handler) {
    if (!this._events.has(type)) this._events.set(type, new Set());
    this._events.get(type).add(handler);
    return () => this.off(type, handler);
  }
  off(type, handler) {
    const set = this._events.get(type);
    if (!set) return;
    set.delete(handler);
    if (set.size === 0) this._events.delete(type);
  }
  emit(type, ...args) {
    const set = this._events.get(type);
    if (!set) return;
    for (const h of [...set]) {
      try { h(...args); } catch (e) { /* swallow */ }
    }
  }
}

/**
 * @typedef {Object} ServifyConfig
 * @property {string} [wsUrl] WebSocket URL. If omitted, deduced from location/baseUrl
 * @property {string} [baseUrl] Server base HTTP URL (e.g. http://localhost:8080)
 * @property {string} [sessionId] Your session id. If omitted, sdk generates one
 * @property {boolean} [autoConnect] default true
 * @property {boolean} [autoReconnect] default true
 * @property {number} [reconnectAttempts] default Infinity
 * @property {number} [reconnectDelayMs] default 500
 * @property {number} [reconnectDelayMaxMs] default 5000
 * @property {number} [heartbeatIntervalMs] default 25000
 * @property {{ urls: string[] }[]} [stunServers]
 */

/**
 * ConnectionStatus: 'idle'|'connecting'|'connected'|'reconnecting'|'disconnected'
 */
export class ServifyClient extends Emitter {
  /**
   * @param {ServifyConfig} config
   */
  constructor(config = {}) {
    super();
    this.config = { ...DEFAULTS, ...config };

    this.sessionId = this.config.sessionId || `web_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    this.status = 'idle';

    // ws state
    this._ws = null;
    this._reconnectAttempt = 0;
    this._hbTimer = null;

    // webrtc state
    this._pc = null;
    this._localStream = null;

    if (this.config.autoConnect) {
      // don't await here; errors will be emitted
      this.connect().catch(() => {});
    }
  }

  /** Resolve WS URL from config or window.location */
  _resolveWsUrl() {
    if (this.config.wsUrl) return this._withSession(this.config.wsUrl);
    // derive from baseUrl or current location
    let base;
    if (this.config.baseUrl) {
      base = new URL(this.config.baseUrl.replace(/\/$/, ''));
    } else if (typeof window !== 'undefined' && window.location) {
      base = new URL(window.location.origin);
    } else {
      throw new Error('Cannot resolve wsUrl without baseUrl in non-browser env');
    }
    const proto = base.protocol === 'https:' ? 'wss:' : 'ws:';
    return this._withSession(`${proto}//${base.host}/api/v1/ws`);
  }

  _withSession(url) {
    const u = new URL(url, typeof window !== 'undefined' ? window.location.href : undefined);
    if (!u.searchParams.get('session_id')) u.searchParams.set('session_id', this.sessionId);
    return u.toString();
  }

  _setStatus(next) {
    if (this.status === next) return;
    this.status = next;
    this.emit('status', next);
  }

  /**
   * Open WebSocket connection
   * @returns {Promise<void>}
   */
  async connect() {
    if (this._ws && (this._ws.readyState === WebSocket.OPEN || this._ws.readyState === WebSocket.CONNECTING)) return;
    const url = this._resolveWsUrl();

    this._setStatus(this.status === 'disconnected' ? 'reconnecting' : 'connecting');

    await new Promise((resolve, reject) => {
      let settled = false;
      try {
        const ws = new WebSocket(url);
        this._ws = ws;

        ws.onopen = () => {
          this._reconnectAttempt = 0;
          this._setStatus('connected');
          this.emit('open');
          this._startHeartbeat();
          if (!settled) { settled = true; resolve(); }
        };
        ws.onclose = (ev) => {
          this._stopHeartbeat();
          this.emit('close', ev);
          // move to disconnected before attempting reconnect (if enabled)
          this._setStatus('disconnected');
          this._maybeReconnect();
          if (!settled) { settled = true; resolve(); }
        };
        ws.onerror = (err) => {
          this.emit('error', err);
          // Some browsers still require reject here to reflect immediate failure
          if (!settled) { settled = true; reject(err); }
        };
        ws.onmessage = (evt) => {
          let payload = null;
          try { payload = JSON.parse(evt.data); } catch { /* ignore */ }
          this.emit('message', payload ?? evt.data);
          // typed helpers
          if (payload && payload.type === 'ai-response') this.emit('ai', payload.data);
          if (payload && payload.type && payload.type.startsWith('webrtc-')) this.emit(`webrtc:${payload.type.slice(7)}`, payload.data);
        };
      } catch (e) {
        if (!settled) { settled = true; reject(e); }
      }
    });
  }

  disconnect(code = 1000, reason = 'client-close') {
    this._stopHeartbeat();
    if (this._ws) {
      try { this._ws.close(code, reason); } catch {}
    }
    this._ws = null;
    this._setStatus('disconnected');
  }

  _startHeartbeat() {
    this._stopHeartbeat();
    if (!this.config.heartbeatIntervalMs) return;
    this._hbTimer = setInterval(() => {
      try { if (this._ws && this._ws.readyState === WebSocket.OPEN) this._ws.send('\n'); } catch {}
    }, this.config.heartbeatIntervalMs);
  }
  _stopHeartbeat() { if (this._hbTimer) { clearInterval(this._hbTimer); this._hbTimer = null; } }

  _maybeReconnect() {
    if (!this.config.autoReconnect) return;
    if (this._reconnectAttempt >= (this.config.reconnectAttempts ?? 0)) return;
    this._reconnectAttempt += 1;
    const d = backoff(this._reconnectAttempt, this.config.reconnectDelayMs, this.config.reconnectDelayMaxMs);
    setTimeout(() => {
      this.connect().catch((e) => this.emit('error', e));
    }, d);
  }

  /** Send any raw payload (object gets JSON.stringified) */
  sendRaw(payload) {
    if (!this._ws || this._ws.readyState !== WebSocket.OPEN) throw new Error('WebSocket not open');
    const data = typeof payload === 'string' ? payload : JSON.stringify(payload);
    this._ws.send(data);
  }

  /** Convenience: send text chat message */
  sendMessage(text) {
    if (typeof text !== 'string' || !text.trim()) throw new Error('message must be non-empty string');
    this.sendRaw({ type: 'text-message', data: { content: text } });
  }

  /** Current status */
  getStatus() { return this.status; }

  // ============ WebRTC (Remote Assist) ============
  /**
   * Start sharing screen via WebRTC (creates local offer and sends over WS)
   * Returns the created RTCPeerConnection
   */
  async startRemoteAssist(constraints = { video: true, audio: false, preferDisplay: true }) {
    if (this._pc) throw new Error('remote assist already active');
    // get media: prefer display capture when requested
    if (constraints.preferDisplay) {
      this._localStream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: false }).catch(() => null);
    }
    if (!this._localStream) {
      this._localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
    }

    const pc = new RTCPeerConnection({ iceServers: this.config.stunServers });
    this._pc = pc;

    // push tracks
    for (const track of this._localStream.getTracks()) pc.addTrack(track, this._localStream);

    // ICE
    pc.onicecandidate = (e) => {
      if (e.candidate) this.sendRaw({ type: 'webrtc-candidate', data: e.candidate });
    };

    pc.onconnectionstatechange = () => {
      this.emit('webrtc:state', pc.connectionState);
      if (pc.connectionState === 'failed' || pc.connectionState === 'closed' || pc.connectionState === 'disconnected') {
        // auto cleanup
        this.endRemoteAssist();
      }
    };

    // Create offer
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    // Send offer over WS; the remote peer should respond with 'webrtc-answer'
    this.sendRaw({ type: 'webrtc-offer', data: offer });

    return pc;
  }

  /**
   * Handle remote answer (if you control both peers in-app)
   */
  async acceptRemoteAnswer(answer) {
    if (!this._pc) throw new Error('no active peer connection');
    await this._pc.setRemoteDescription(new RTCSessionDescription(answer));
  }

  /** Pass through ICE candidate from remote */
  async addRemoteIce(candidate) {
    if (!this._pc) return;
    try { await this._pc.addIceCandidate(new RTCIceCandidate(candidate)); } catch {}
  }

  /** Stop screen sharing and close peer */
  endRemoteAssist() {
    if (this._pc) {
      try { this._pc.close(); } catch {}
      this._pc = null;
    }
    if (this._localStream) {
      try { this._localStream.getTracks().forEach(t => t.stop()); } catch {}
      this._localStream = null;
    }
    this.emit('webrtc:state', 'closed');
  }
}

/** Factory */
export function createClient(config) { return new ServifyClient(config); }

export default ServifyClient;
