import { h } from 'preact';

export function TokenTicker({ tokenCounts }) {
  const formatNum = (num) => (num || 0).toLocaleString();
  
  const inputText = tokenCounts.inputText || 0;
  const inputAudio = tokenCounts.inputAudio || 0;
  const outputText = tokenCounts.outputText || 0;
  const outputAudio = tokenCounts.outputAudio || 0;

  const totalBilled = inputText + inputAudio + outputText + outputAudio;

  // Gemini 2.5 Flash Live API Rates
  // Input: Text $0.50 / 1M, Audio $3.00 / 1M
  // Output: Text $2.00 / 1M, Audio $12.00 / 1M
  const inputTextCost = inputText * (0.50 / 1000000);
  const inputAudioCost = inputAudio * (3.00 / 1000000);
  const outputTextCost = outputText * (2.00 / 1000000);
  const outputAudioCost = outputAudio * (12.00 / 1000000);

  const totalCost = inputTextCost + inputAudioCost + outputTextCost + outputAudioCost;

  const formatCost = (cost) => {
    if (!cost || cost === 0) return '$0.0000';
    if (cost < 0.0001) return `$${cost.toFixed(6)}`;
    if (cost < 0.01) return `$${cost.toFixed(4)}`;
    return `$${cost.toFixed(2)}`;
  };

  return (
    <div className="glass-panel" style={{ padding: '1rem', display: 'flex', flexDirection: 'column', gap: '0.75rem', overflow: 'hidden' }}>
      {/* Above Ticker: Pricing Rate Assumption Header */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.2rem' }}>
        <div style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-bright)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span>Token Telemetry</span>
          <span className="status-pill" style={{ fontSize: '0.7rem', color: 'var(--color-primary)', background: 'rgba(0, 240, 255, 0.1)', borderColor: 'rgba(0, 240, 255, 0.2)' }}>
            {formatNum(totalBilled)} Billed
          </span>
        </div>
        <div style={{ fontSize: '0.65rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          Gemini 2.5 Flash Live Rates: $3.00/1M Aud In • $12.00/1M Aud Out
        </div>
      </div>

      {/* Main Ticker Grid */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem', fontSize: '0.75rem' }}>
        <div style={{ background: 'rgba(0,0,0,0.2)', padding: '0.5rem 0.75rem', borderRadius: 'var(--border-radius-sm)', border: '1px solid var(--border-color)', minWidth: 0 }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.65rem', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            INPUT (TXT / AUD)
          </div>
          <div style={{ color: 'var(--color-secondary)', fontWeight: 600, fontFamily: 'var(--font-mono)', marginTop: '0.2rem', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {formatNum(inputText)} / {formatNum(inputAudio)}
          </div>
        </div>

        <div style={{ background: 'rgba(0,0,0,0.2)', padding: '0.5rem 0.75rem', borderRadius: 'var(--border-radius-sm)', border: '1px solid var(--border-color)', minWidth: 0 }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.65rem', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            OUTPUT (TXT / AUD)
          </div>
          <div style={{ color: 'var(--color-primary)', fontWeight: 600, fontFamily: 'var(--font-mono)', marginTop: '0.2rem', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {formatNum(outputText)} / {formatNum(outputAudio)}
          </div>
        </div>
      </div>

      {/* Below Ticker: Total Session Cost Counter */}
      <div style={{ borderTop: '1px solid var(--border-color)', paddingTop: '0.6rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '0.75rem' }}>
        <span style={{ color: 'var(--text-muted)', fontSize: '0.7rem', fontWeight: 500 }}>
          Est. Session Cost
        </span>
        <span
          title={`Exact Cost: $${totalCost.toFixed(8)}`}
          style={{
            color: '#00e676',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            fontSize: '0.85rem',
            background: 'rgba(0, 230, 118, 0.1)',
            padding: '0.2rem 0.5rem',
            borderRadius: 'var(--border-radius-sm)',
            border: '1px solid rgba(0, 230, 118, 0.2)',
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            maxWidth: '120px'
          }}
        >
          {formatCost(totalCost)}
        </span>
      </div>
    </div>
  );
}
