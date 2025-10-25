/*
 Servify Web SDK (UMD)
 Attaches to window.Servify = { ServifyClient, createClient }
*/
(function (root, factory) {
  if (typeof define === 'function' && define.amd) {
    define([], factory);
  } else if (typeof module === 'object' && module.exports) {
    module.exports = factory();
  } else {
    root.Servify = factory();
  }
}(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  var DEFAULTS = {
    autoConnect: true,
    autoReconnect: true,
    reconnectAttempts: Infinity,
    reconnectDelayMs: 500,
    reconnectDelayMaxMs: 5000,
    heartbeatIntervalMs: 25000,
    stunServers: [ { urls: ['stun:stun.l.google.com:19302'] } ],
  };
  function backoff(attempt, base, max) {
    var jitter = Math.random() * 0.2 + 0.9;
    var delay = Math.min(max, base * Math.pow(2, Math.max(0, attempt - 1)));
    return Math.floor(delay * jitter);
  }
  function Emitter() { this._events = new Map(); }
  Emitter.prototype.on = function (type, handler) {
    if (!this._events.has(type)) this._events.set(type, new Set());
    this._events.get(type).add(handler);
    var self = this;
    return function () { self.off(type, handler); };
  };
  Emitter.prototype.off = function (type, handler) {
    var set = this._events.get(type); if (!set) return; set.delete(handler); if (set.size === 0) this._events.delete(type);
  };
  Emitter.prototype.emit = function (type) {
    var set = this._events.get(type); if (!set) return;
    var args = Array.prototype.slice.call(arguments, 1);
    set.forEach(function (h) { try { h.apply(null, args); } catch (e) {} });
  };

  function ServifyClient(config) {
    Emitter.call(this);
    this.config = Object.assign({}, DEFAULTS, config || {});
    this.sessionId = this.config.sessionId || ('web_' + Date.now() + '_' + Math.random().toString(36).slice(2, 8));
    this.status = 'idle';
    this._ws = null; this._reconnectAttempt = 0; this._hbTimer = null; this._pc = null; this._localStream = null;
    if (this.config.autoConnect) { var _ = this.connect().catch(function(){}) }
  }
  ServifyClient.prototype = Object.create(Emitter.prototype);
  ServifyClient.prototype.constructor = ServifyClient;

  ServifyClient.prototype._resolveWsUrl = function () {
    if (this.config.wsUrl) return this._withSession(this.config.wsUrl);
    var base;
    if (this.config.baseUrl) {
      base = new URL(this.config.baseUrl.replace(/\/$/, ''));
    } else if (typeof window !== 'undefined' && window.location) {
      base = new URL(window.location.origin);
    } else {
      throw new Error('Cannot resolve wsUrl without baseUrl in non-browser env');
    }
    var proto = base.protocol === 'https:' ? 'wss:' : 'ws:';
    return this._withSession(proto + '//' + base.host + '/api/v1/ws');
  };
  ServifyClient.prototype._withSession = function (url) {
    var u = new URL(url, typeof window !== 'undefined' ? window.location.href : undefined);
    if (!u.searchParams.get('session_id')) u.searchParams.set('session_id', this.sessionId);
    return u.toString();
  };
  ServifyClient.prototype._setStatus = function (next) { if (this.status === next) return; this.status = next; this.emit('status', next); };

  ServifyClient.prototype.connect = function () {
    var self = this;
    if (self._ws && (self._ws.readyState === WebSocket.OPEN || self._ws.readyState === WebSocket.CONNECTING)) return Promise.resolve();
    var url = self._resolveWsUrl();
    self._setStatus(self.status === 'disconnected' ? 'reconnecting' : 'connecting');
    return new Promise(function (resolve, reject) {
      var settled = false;
      try {
        var ws = new WebSocket(url); self._ws = ws;
        ws.onopen = function () { self._reconnectAttempt = 0; self._setStatus('connected'); self.emit('open'); self._startHeartbeat(); if (!settled) { settled = true; resolve(); } };
        ws.onclose = function (ev) { self._stopHeartbeat(); self.emit('close', ev); self._setStatus('disconnected'); self._maybeReconnect(); if (!settled) { settled = true; resolve(); } };
        ws.onerror = function (err) { self.emit('error', err); if (!settled) { settled = true; reject(err); } };
        ws.onmessage = function (evt) {
          var payload = null; try { payload = JSON.parse(evt.data); } catch (e) {}
          self.emit('message', payload || evt.data);
          if (payload && payload.type === 'ai-response') self.emit('ai', payload.data);
          if (payload && payload.type && payload.type.indexOf('webrtc-') === 0) self.emit('webrtc:' + payload.type.slice(7), payload.data);
        };
      } catch (e) { if (!settled) { settled = true; reject(e); } }
    });
  };

  ServifyClient.prototype.disconnect = function (code, reason) {
    if (code === void 0) code = 1000; if (reason === void 0) reason = 'client-close';
    this._stopHeartbeat(); if (this._ws) { try { this._ws.close(code, reason); } catch (e) {} } this._ws = null; this._setStatus('disconnected');
  };
  ServifyClient.prototype._startHeartbeat = function () {
    var self = this; self._stopHeartbeat(); if (!self.config.heartbeatIntervalMs) return;
    self._hbTimer = setInterval(function () { try { if (self._ws && self._ws.readyState === WebSocket.OPEN) self._ws.send('\n'); } catch (e) {} }, self.config.heartbeatIntervalMs);
  };
  ServifyClient.prototype._stopHeartbeat = function () { if (this._hbTimer) { clearInterval(this._hbTimer); this._hbTimer = null; } };
  ServifyClient.prototype._maybeReconnect = function () {
    if (!this.config.autoReconnect) return; if (this._reconnectAttempt >= (this.config.reconnectAttempts || 0)) return;
    this._reconnectAttempt += 1; var d = backoff(this._reconnectAttempt, this.config.reconnectDelayMs, this.config.reconnectDelayMaxMs);
    var self = this; setTimeout(function () { self.connect().catch(function (e) { self.emit('error', e); }); }, d);
  };

  ServifyClient.prototype.sendRaw = function (payload) {
    if (!this._ws || this._ws.readyState !== WebSocket.OPEN) throw new Error('WebSocket not open');
    var data = typeof payload === 'string' ? payload : JSON.stringify(payload); this._ws.send(data);
  };
  ServifyClient.prototype.sendMessage = function (text) {
    if (typeof text !== 'string' || !text.trim()) throw new Error('message must be non-empty string');
    this.sendRaw({ type: 'text-message', data: { content: text } });
  };
  ServifyClient.prototype.getStatus = function () { return this.status; };

  // WebRTC
  ServifyClient.prototype.startRemoteAssist = function (constraints) {
    constraints = constraints || { video: true, audio: false, preferDisplay: true };
    var self = this; if (self._pc) return Promise.reject(new Error('remote assist already active'));
    return Promise.resolve().then(function () {
      if (constraints.preferDisplay) {
        return navigator.mediaDevices.getDisplayMedia({ video: true, audio: false }).catch(function () { return null; });
      }
      return null;
    }).then(function (stream) {
      if (!stream) { return navigator.mediaDevices.getUserMedia({ video: true, audio: false }); }
      return stream;
    }).then(function (stream) {
      self._localStream = stream;
      var pc = new RTCPeerConnection({ iceServers: self.config.stunServers }); self._pc = pc;
      self._localStream.getTracks().forEach(function (t) { pc.addTrack(t, self._localStream); });
      pc.onicecandidate = function (e) { if (e.candidate) self.sendRaw({ type: 'webrtc-candidate', data: e.candidate }); };
      pc.onconnectionstatechange = function () {
        self.emit('webrtc:state', pc.connectionState);
        if (pc.connectionState === 'failed' || pc.connectionState === 'closed' || pc.connectionState === 'disconnected') self.endRemoteAssist();
      };
      return pc.createOffer().then(function (offer) { return pc.setLocalDescription(offer).then(function () { return offer; }); });
    }).then(function (offer) {
      self.sendRaw({ type: 'webrtc-offer', data: offer });
      return self._pc;
    });
  };
  ServifyClient.prototype.acceptRemoteAnswer = function (answer) {
    if (!this._pc) return Promise.reject(new Error('no active peer connection'));
    return this._pc.setRemoteDescription(new RTCSessionDescription(answer));
  };
  ServifyClient.prototype.addRemoteIce = function (candidate) {
    if (!this._pc) return Promise.resolve();
    return this._pc.addIceCandidate(new RTCIceCandidate(candidate)).catch(function () {});
  };
  ServifyClient.prototype.endRemoteAssist = function () {
    if (this._pc) { try { this._pc.close(); } catch (e) {} this._pc = null; }
    if (this._localStream) { try { this._localStream.getTracks().forEach(function (t) { t.stop(); }); } catch (e) {} this._localStream = null; }
    this.emit('webrtc:state', 'closed');
  };

  function createClient(config) { return new ServifyClient(config); }

  return { ServifyClient: ServifyClient, createClient: createClient };
}));
