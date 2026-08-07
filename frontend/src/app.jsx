import { h, Fragment } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';
import { AudioManager } from './utils/audio';
import { WSClient } from './utils/websocket';
import { Visualizer } from './components/Visualizer';
import { ChatLog, ToolInspector } from './components/ChatLog';
import { TokenTicker } from './components/TokenTicker';

export function App() {
  const [isConnected, setIsConnected] = useState(false);
  const [connStatus, setConnectionStatus] = useState('disconnected');
  const [isRecording, setIsRecording] = useState(false);
  const [isSpeaking, setIsSpeaking] = useState(false);
  const [messages, setMessages] = useState([]);
  const [toolsLog, setToolsLog] = useState([]);
  const [systemLogs, setSystemLogs] = useState([]);
  const [inputText, setTextInputValue] = useState('');
  const [serverUrl, setServerUrl] = useState('');
  const [sessionId, setSessionId] = useState('');
  const [tokenCounts, setTokenCounts] = useState({
    inputText: 0,
    inputAudio: 0,
    cachedText: 0,
    cachedAudio: 0,
    outputText: 0,
    outputAudio: 0,
  });

  const audioManagerRef = useRef(null);
  const wsClientRef = useRef(null);

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host || 'localhost:8080';
    setServerUrl(`${protocol}//${host}/ws`);

    audioManagerRef.current = new AudioManager();

    return () => {
      if (audioManagerRef.current) {
        audioManagerRef.current.close();
      }
      if (wsClientRef.current) {
        wsClientRef.current.disconnect();
      }
    };
  }, []);

  const handleToggleConnect = async () => {
    if (isConnected) {
      handleDisconnect();
      return;
    }

    try {
      addSystemLog('Connecting to proxy server...');
      setTokenCounts({
        inputText: 0,
        inputAudio: 0,
        cachedText: 0,
        cachedAudio: 0,
        outputText: 0,
        outputAudio: 0,
      });

      await audioManagerRef.current.init();

      const ws = new WSClient(serverUrl);
      wsClientRef.current = ws;

      ws.on('open', () => {
        addSystemLog('WebSocket connection opened');
        addEventMessage('connection', 'WebSocket Connection Opened');
      });

      ws.on('close', () => {
        addSystemLog('WebSocket connection closed');
        addEventMessage('connection', 'WebSocket Connection Closed');
        handleDisconnect();
      });

      ws.on('error', (err) => {
        const errMsg = err.message || 'Unknown';
        addSystemLog(`Connection error: ${errMsg}`);
        addEventMessage('error', `Connection Error: ${errMsg}`);
      });

      ws.on('status', (msg) => {
        const text = msg.text || msg.Text || '';
        const sId = msg.sessionId || msg.SessionID || '';
        setSessionId(sId);
        setIsConnected(true);
        setConnectionStatus(text);
        const isActiveLive = text === 'connected_live';
        addSystemLog(`Session active: ${isActiveLive ? 'Live GCloud stream' : 'Offline Emulator sandbox'}`);
        addEventMessage('connection', `Session Active: ${isActiveLive ? 'Live GCloud Stream (Aoede)' : 'Offline Emulator Sandbox'}`);
        startMicrophoneStream(ws);
      });

      ws.on('audio', (msg) => {
        const data = msg.data || msg.Data || '';
        if (data && audioManagerRef.current) {
          audioManagerRef.current.playPCMChunk(data);
          setIsSpeaking(true);
        }
      });

      ws.on('audio_done', () => {
        setIsSpeaking(false);
      });

      ws.on('control', (msg) => {
        const text = msg.text || msg.Text || '';
        if (text === 'clear' && audioManagerRef.current) {
          audioManagerRef.current.clearPlaybackQueue();
          setIsSpeaking(false);
          addSystemLog('Playback interrupted and cleared');
          addEventMessage('interrupt', 'Playback Interrupted and Cleared');
        }
      });

      ws.on('stream_transcript_assistant', (msg) => {
        handleIncrementalTranscript('assistant', msg.text || msg.Text || '');
      });

      ws.on('stream_transcript_user', (msg) => {
        handleIncrementalTranscript('user', msg.text || msg.Text || '');
      });

      ws.on('text', (msg) => {
        handleIncrementalTranscript('assistant', msg.text || msg.Text || '');
      });

      ws.on('transcript_user', (msg) => {
        finalizeTranscript('user', msg.text || msg.Text || '');
      });

      ws.on('transcript_assistant', (msg) => {
        finalizeTranscript('assistant', msg.text || msg.Text || '');
      });

      ws.on('tool_call', (msg) => {
        const toolName = msg.toolName || msg.ToolName || '';
        const toolArgs = msg.toolArgs || msg.ToolArgs || {};
        const id = 'tool-' + Date.now();
        setToolsLog(prev => [...prev, { id, name: toolName, args: toolArgs, status: 'calling', result: null }]);
        addSystemLog(`Tool invoked: ${toolName}`);
        addEventMessage('tool_call', `Tool Invoked: ${toolName}`);
      });

      ws.on('tool_response', (msg) => {
        const toolName = msg.toolName || msg.ToolName || '';
        const resultText = msg.text || msg.Text || msg.result || '';
        setToolsLog(prev => {
          const copy = [...prev];
          const idx = findLastIndex(copy, t => t.name === toolName && t.status === 'calling');
          if (idx > -1) {
            copy[idx] = { ...copy[idx], status: 'complete', result: resultText };
          }
          return copy;
        });
        addSystemLog(`Tool execution finished: ${toolName}`);
        addEventMessage('tool_response', `Tool Result: ${toolName}`);
      });

      ws.on('metrics', (msg) => {
        const tokenDetails = msg.tokenDetails || msg.TokenDetails || [];
        if (tokenDetails && tokenDetails.length > 0) {
          setTokenCounts(prev => {
            const updated = { ...prev };
            tokenDetails.forEach(detail => {
              const type = detail.type || detail.Type || '';
              const modality = detail.modality || detail.Modality || '';
              const count = detail.count !== undefined ? detail.count : (detail.Count !== undefined ? detail.Count : 0);
              if (type === 'input') {
                if (modality === 'text') updated.inputText = (prev.inputText || 0) + count;
                else if (modality === 'audio') updated.inputAudio = (prev.inputAudio || 0) + count;
              } else if (type === 'cached') {
                if (modality === 'text') updated.cachedText = (prev.cachedText || 0) + count;
                else if (modality === 'audio') updated.cachedAudio = (prev.cachedAudio || 0) + count;
              } else if (type === 'output') {
                if (modality === 'text') updated.outputText = (prev.outputText || 0) + count;
                else if (modality === 'audio') updated.outputAudio = (prev.outputAudio || 0) + count;
              }
            });
            return updated;
          });
        }
      });

      ws.on('end_call', () => {
        addSystemLog('Call ended by assistant');
        addEventMessage('connection', 'Call Ended by Assistant');
        handleDisconnect();
      });

      await ws.connect();
    } catch (err) {
      console.error(err);
      addSystemLog(`Failed to establish connection: ${err.message || err}`);
      handleDisconnect();
    }
  };

  const handleDisconnect = () => {
    setIsConnected(false);
    setConnectionStatus('disconnected');
    setIsRecording(false);
    setIsSpeaking(false);
    setSessionId('');
    if (audioManagerRef.current) {
      audioManagerRef.current.stopRecording();
      audioManagerRef.current.clearPlaybackQueue();
    }
    if (wsClientRef.current) {
      wsClientRef.current.disconnect();
      wsClientRef.current = null;
    }
    addSystemLog('Session ended');
  };

  const startMicrophoneStream = async (wsClient) => {
    try {
      if (!audioManagerRef.current) return;
      addSystemLog('Starting microphone stream (16kHz mono)...');
      setIsRecording(true);
      await audioManagerRef.current.startRecording((pcmChunkBuffer) => {
        if (wsClient) {
          wsClient.sendAudioFrame(pcmChunkBuffer);
        }
      });
    } catch (err) {
      console.error('Mic capture failed', err);
      addSystemLog(`Microphone capture blocked: ${err.message || err}`);
      setIsRecording(false);
    }
  };

  const handleToggleMic = async () => {
    if (isRecording) {
      if (audioManagerRef.current) {
        audioManagerRef.current.stopRecording();
      }
      setIsRecording(false);
      addSystemLog('Microphone muted');
    } else {
      if (wsClientRef.current) {
        await startMicrophoneStream(wsClientRef.current);
      }
    }
  };

  const handleForceInterrupt = () => {
    if (wsClientRef.current) {
      wsClientRef.current.sendInterrupt();
      if (audioManagerRef.current) {
        audioManagerRef.current.clearPlaybackQueue();
      }
      setIsSpeaking(false);
    }
  };

  const handleSendTextQuery = (e) => {
    e.preventDefault();
    if (!inputText.trim()) return;
    if (wsClientRef.current) {
      wsClientRef.current.sendTextInput(inputText.trim());
      const userMsgId = 'user-' + Date.now();
      const currentTimestamp = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
      setMessages(prev => [
        ...prev,
        { id: userMsgId, role: 'user', text: inputText.trim(), isFinal: true, timestamp: currentTimestamp }
      ]);
      setTextInputValue('');
    }
  };

  const handleIncrementalTranscript = (role, chunkText = '') => {
    if (!chunkText) return;
    setMessages((prev) => {
      const copy = [...prev];
      const idx = findLastIndex(copy, m => m.role === role && !m.isFinal);
      if (idx > -1) {
        copy[idx] = { ...copy[idx], text: (copy[idx].text || '') + chunkText };
        return copy;
      } else {
        const currentTimestamp = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        return [...prev, { id: role + '-' + Date.now(), role, text: chunkText, isFinal: false, timestamp: currentTimestamp }];
      }
    });
  };

  const finalizeTranscript = (role, finalFullText = '') => {
    setMessages((prev) => {
      const copy = [...prev];
      let idx = findLastIndex(copy, m => m.role === role && !m.isFinal);
      if (idx === -1) {
        idx = findLastIndex(copy, m => m.role === role);
      }
      if (idx > -1) {
        const updatedText = finalFullText || copy[idx].text || '';
        if (!updatedText.trim()) {
          return copy.filter((_, i) => i !== idx);
        }
        copy[idx] = { ...copy[idx], text: updatedText, isFinal: true };
        return copy;
      } else if (finalFullText && finalFullText.trim()) {
        const currentTimestamp = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        return [...prev, { id: role + '-' + Date.now(), role, text: finalFullText, isFinal: true, timestamp: currentTimestamp }];
      }
      return prev;
    });
  };

  const addSystemLog = (text) => {
    const timestamp = new Date().toLocaleTimeString();
    setSystemLogs(prev => [...prev, `[${timestamp}] ${text}`].slice(-150));
  };

  const addEventMessage = (type, text) => {
    const timestamp = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    setMessages(prev => [
      ...prev,
      {
        id: 'event-' + Date.now(),
        role: 'event',
        type,
        text,
        timestamp,
        isFinal: true
      }
    ]);
  };

  const findLastIndex = (arr, predicate) => {
    for (let i = arr.length - 1; i >= 0; i--) {
      if (predicate(arr[i])) return i;
    }
    return -1;
  };

  return (
    <Fragment>
      <header className="glass-panel" style={{ margin: '1rem 2rem 0', borderRadius: 'var(--border-radius-lg)' }}>
        <div className="logo-container">
          <div className="logo-glow" />
          <h1 className="logo-text" style={{ margin: 0, fontSize: '1.25rem' }}>RiderMate Voice Assistant</h1>
        </div>
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
          <span className={`status-pill ${connStatus === 'connected_live' ? 'status-connected' : (connStatus === 'connected_sandbox' ? 'status-simulated' : 'status-disconnected')}`}>
            <span className="status-dot" />
            {connStatus === 'connected_live' ? 'GCloud Active' : (connStatus === 'connected_sandbox' ? 'Sandbox Emulator' : 'Disconnected')}
          </span>
        </div>
      </header>

      <main className="dashboard-grid">
        <div className="main-column">
          <div className="controls-card glass-panel">
            <div className="input-group">
              <input
                type="text"
                value={serverUrl}
                onChange={(e) => setServerUrl(e.target.value)}
                placeholder="ws://localhost:8080/ws"
                disabled={isConnected}
              />
            </div>
            
            <button
              onClick={handleToggleConnect}
              className={`btn ${isConnected ? 'btn-danger' : 'btn-primary'}`}
            >
              {isConnected ? 'Disconnect Stream' : 'Connect Stream'}
            </button>

            {isConnected && (
              <Fragment>
                <button
                  onClick={handleToggleMic}
                  className={`btn btn-secondary`}
                  style={{ borderColor: isRecording ? 'rgba(0, 230, 118, 0.4)' : 'var(--border-color)' }}
                >
                  {isRecording ? '🎙️ Mute Microphone' : '🔇 Unmute Microphone'}
                </button>
                <button
                  onClick={handleForceInterrupt}
                  className="btn btn-secondary"
                  style={{ color: 'var(--color-warning)' }}
                >
                  ⚡ Interrupt
                </button>
                <button
                  onClick={() => wsClientRef.current?.sendForceEndCall()}
                  className="btn btn-secondary"
                  style={{ color: 'var(--color-error)' }}
                >
                  ❌ End Session
                </button>
              </Fragment>
            )}
          </div>

          <div className="visualizer-card glass-panel">
            <div className="visualizer-header">
              <div className="visualizer-title">Live Voice Equalizer</div>
              <div style={{ display: 'flex', gap: '1rem', fontSize: '0.75rem' }}>
                <span style={{ color: isRecording ? '#00e676' : 'var(--text-muted)' }}>
                  ● Mic Stream {isRecording ? 'Active' : 'Muted'}
                </span>
                <span style={{ color: isSpeaking ? '#00f0ff' : 'var(--text-muted)' }}>
                  ● Assistant {isSpeaking ? 'Speaking' : 'Idle'}
                </span>
              </div>
            </div>
            <Visualizer
              audioManager={audioManagerRef.current}
              isRecording={isRecording}
              isSpeaking={isSpeaking}
            />
          </div>

          <ChatLog messages={messages} isAssistantTyping={isSpeaking && messages.length > 0 && !messages[messages.length - 1].isFinal} sessionId={sessionId} />

          {isConnected && (
            <form onSubmit={handleSendTextQuery} className="glass-panel" style={{ padding: '0.75rem', display: 'flex', gap: '0.5rem', borderRadius: 'var(--border-radius-md)' }}>
              <input
                type="text"
                value={inputText}
                onChange={(e) => setTextInputValue(e.target.value)}
                placeholder="Type a rider support query (e.g. 'cancel order #RD892', 'check today's earnings', 'transfer to rider support')..."
                style={{
                  flex: 1,
                  background: 'rgba(0,0,0,0.2)',
                  border: '1px solid var(--border-color)',
                  borderRadius: 'var(--border-radius-sm)',
                  padding: '0.65rem 1rem',
                  color: 'white',
                  outline: 'none',
                }}
              />
              <button type="submit" className="btn btn-primary" style={{ padding: '0.5rem 1.25rem' }}>
                Send Msg
              </button>
            </form>
          )}
        </div>

        <div className="sidebar-column">
          <ToolInspector tools={toolsLog} />
          <TokenTicker tokenCounts={tokenCounts} />
          <div className="glass-panel" style={{ flex: 'none', height: '220px', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
            <div style={{ padding: '0.75rem 1rem', borderBottom: '1px solid var(--border-color)', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-bright)' }}>
              System Event Monitor
            </div>
            <div
              style={{
                flex: 1,
                padding: '0.75rem 1rem',
                fontFamily: 'var(--font-mono)',
                fontSize: '0.75rem',
                color: 'var(--text-muted)',
                backgroundColor: 'rgba(0, 0, 0, 0.2)',
                overflowY: 'auto',
                display: 'flex',
                flexDirection: 'column',
                gap: '0.2rem',
              }}
              ref={(el) => { if (el) el.scrollTop = el.scrollHeight; }}
            >
              {systemLogs.length === 0 ? (
                <div style={{ fontStyle: 'italic', opacity: 0.5 }}>System status normal. Standing by...</div>
              ) : (
                systemLogs.map((log, i) => <div key={i}>{log}</div>)
              )}
            </div>
          </div>
        </div>
      </main>
    </Fragment>
  );
}
