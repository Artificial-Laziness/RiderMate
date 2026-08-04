import { AudioManager } from './audio';

export class WSClient {
  constructor(url) {
    this.url = url;
    this.ws = null;
    this.listeners = {};
  }

  on(event, callback) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(callback);
  }

  emit(event, data) {
    if (this.listeners[event]) {
      this.listeners[event].forEach(cb => cb(data));
    }
  }

  connect() {
    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(this.url);
        this.ws.binaryType = 'arraybuffer';

        this.ws.onopen = () => {
          this.emit('open');
          resolve();
        };

        this.ws.onclose = () => {
          this.emit('close');
        };

        this.ws.onerror = (err) => {
          this.emit('error', err);
          reject(err);
        };

        this.ws.onmessage = (event) => {
          if (typeof event.data === 'string') {
            try {
              const msg = JSON.parse(event.data);
              if (msg.type) {
                this.emit(msg.type, msg);
              }
              this.emit('message', msg);
            } catch (e) {
              console.error('Failed to parse WS JSON', e);
            }
          }
        };
      } catch (err) {
        reject(err);
      }
    });
  }

  sendAudioFrame(pcmArrayBuffer) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(pcmArrayBuffer);
    }
  }

  sendTextInput(text) {
    this.sendJSON({ type: 'text_input', text });
  }

  sendInterrupt() {
    this.sendJSON({ type: 'interrupt' });
  }

  sendForceEndCall() {
    this.sendJSON({ type: 'force_end_call' });
  }

  sendJSON(obj) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(obj));
    }
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}
