import { h } from 'preact';

export function TokenTicker({ tokenCounts }) {
  const formatNum = (num) => (num || 0).toLocaleString();

  return (
    <div className="glass-panel" style={{ padding: '1rem', display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
      <div style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-bright)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span>Token Telemetry</span>
        <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>Gemini Live Modality</span>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem', fontSize: '0.75rem' }}>
        <div style={{ background: 'rgba(0,0,0,0.2)', padding: '0.5rem 0.75rem', borderRadius: 'var(--border-radius-sm)', border: '1px solid var(--border-color)' }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.65rem' }}>INPUT (TXT / AUD)</div>
          <div style={{ color: 'var(--color-secondary)', fontWeight: 600, fontFamily: 'var(--font-mono)', marginTop: '0.2rem' }}>
            {formatNum(tokenCounts.inputText)} / {formatNum(tokenCounts.inputAudio)}
          </div>
        </div>

        <div style={{ background: 'rgba(0,0,0,0.2)', padding: '0.5rem 0.75rem', borderRadius: 'var(--border-radius-sm)', border: '1px solid var(--border-color)' }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.65rem' }}>OUTPUT (TXT / AUD)</div>
          <div style={{ color: 'var(--color-primary)', fontWeight: 600, fontFamily: 'var(--font-mono)', marginTop: '0.2rem' }}>
            {formatNum(tokenCounts.outputText)} / {formatNum(tokenCounts.outputAudio)}
          </div>
        </div>
      </div>
    </div>
  );
}
