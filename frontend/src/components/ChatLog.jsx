import { h } from 'preact';
import { useEffect, useRef } from 'preact/hooks';

export function ChatLog({ messages, isAssistantTyping, sessionId }) {
  const containerRef = useRef(null);

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [messages, isAssistantTyping]);

  return (
    <div className="chat-card glass-panel">
      <div className="chat-header">
        <h2 style={{ fontSize: '1rem', margin: 0, fontWeight: 600 }}>Conversation Stream</h2>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
          {sessionId && (
            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginRight: '0.5rem' }}>
              Session: <code style={{ color: 'var(--text-bright)', background: 'rgba(255,255,255,0.05)', padding: '0.1rem 0.3rem', borderRadius: '3px' }}>{sessionId}</code>
            </span>
          )}
          <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
            {messages.length} messages
          </span>
        </div>
      </div>
      
      <div className="chat-messages" ref={containerRef}>
        {messages.length === 0 ? (
          <div className="empty-state">
            <div className="empty-icon">🎙️</div>
            <p style={{ fontWeight: 500, color: 'var(--text-bright)' }}>Start the stream to begin conversation</p>
            <p style={{ fontSize: '0.85rem', marginTop: '0.25rem' }}>Speak into your mic or type a text message below.</p>
          </div>
        ) : (
          messages.map((msg) => {
            if (msg.role === 'event') {
              return (
                <div
                  key={msg.id}
                  className={`message message-event message-event-${msg.type || 'generic'}`}
                >
                  <div className="message-bubble">
                    <span className="event-icon">
                      {msg.type === 'connection' && '🌐'}
                      {msg.type === 'tool_call' && '⚙️'}
                      {msg.type === 'tool_response' && '✅'}
                      {msg.type === 'interrupt' && '⚡'}
                      {msg.type === 'cta' && '📢'}
                      {msg.type === 'error' && '⚠️'}
                    </span>
                    <span>{msg.text}</span>
                    {msg.timestamp && (
                      <span className="message-timestamp" style={{ marginLeft: 'auto', paddingLeft: '0.5rem' }}>
                        {msg.timestamp}
                      </span>
                    )}
                  </div>
                </div>
              );
            }
            return (
              <div
                key={msg.id}
                className={`message ${msg.role === 'user' ? 'message-user' : 'message-assistant'}`}
              >
                <div className="message-sender" style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                  <span>{msg.role === 'user' ? 'You' : 'Gemini'}</span>
                  {msg.timestamp && (
                    <span className="message-timestamp">
                      {msg.timestamp}
                    </span>
                  )}
                </div>
                <div className="message-bubble">
                  {msg.text}
                  {!msg.isFinal && <span style={{ opacity: 0.5, fontStyle: 'italic' }}>...</span>}
                </div>
              </div>
            );
          })
        )}

        {isAssistantTyping && (
          <div className="message message-assistant">
            <div className="message-sender">Gemini</div>
            <div className="message-bubble" style={{ padding: '0.5rem 1rem' }}>
              <div className="typing-indicator">
                <div className="typing-dot" />
                <div className="typing-dot" />
                <div className="typing-dot" />
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export function ToolInspector({ tools }) {
  const containerRef = useRef(null);

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [tools]);

  return (
    <div className="tools-card glass-panel">
      <div className="tools-header">
        <h2 style={{ fontSize: '1rem', margin: 0, fontWeight: 600 }}>Active Tools Log</h2>
        <span className="status-pill" style={{ padding: '0.15rem 0.5rem', fontSize: '0.7rem' }}>
          {tools.filter(t => t.status === 'complete').length}/{tools.length} Ok
        </span>
      </div>

      <div className="tools-list" ref={containerRef}>
        {tools.length === 0 ? (
          <div className="empty-state" style={{ padding: '2rem' }}>
            <div className="empty-icon" style={{ fontSize: '1.75rem' }}>🛠️</div>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>No tool invocations recorded yet</p>
            <p style={{ fontSize: '0.75rem', marginTop: '0.25rem' }}>Ask RiderMate to cancel an active order, check weekly payouts, or connect to Rider Support to trigger tools.</p>
          </div>
        ) : (
          tools.map((t) => (
            <div key={t.id} className="tool-item">
              <div className="tool-meta">
                <span className="tool-name">{t.name}</span>
                <span className={`tool-status ${t.status === 'calling' ? 'status-call' : 'status-complete'}`}>
                  {t.status === 'calling' ? 'Calling...' : 'Success'}
                </span>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>Arguments:</div>
                <pre className="tool-json">{JSON.stringify(t.args, null, 2)}</pre>
              </div>
              {t.result && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', marginTop: '0.5rem' }}>
                  <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>Response:</div>
                  <pre className="tool-json" style={{ borderColor: 'rgba(16, 185, 129, 0.2)', color: 'var(--text-main)' }}>
                    {typeof t.result === 'object' ? JSON.stringify(t.result, null, 2) : t.result}
                  </pre>
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
