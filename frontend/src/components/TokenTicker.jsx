import { h } from 'preact';
import { useState } from 'preact/hooks';

export function TokenTicker({ tokenCounts }) {
  const [mode, setMode] = useState('normalized'); // 'normalized' | 'naive'
  const formatNum = (num) => (num || 0).toLocaleString();

  // Normalized Watermark (True Context)
  const normInputText = tokenCounts.inputText || 0;
  const normInputAudio = tokenCounts.inputAudio || 0;
  const normTotalInput = normInputText + normInputAudio;

  // Naive Event-Summed (Uncorrected Bug Trap)
  const naiveInputText = tokenCounts.naiveInputText || 0;
  const naiveInputAudio = tokenCounts.naiveInputAudio || 0;
  const naiveTotalInput = naiveInputText + naiveInputAudio;

  // Shared Output Tokens (Incremental Deltas)
  const outputText = tokenCounts.outputText || 0;
  const outputAudio = tokenCounts.outputAudio || 0;
  const totalOutput = outputText + outputAudio;

  const eventsCount = tokenCounts.eventsCount || 0;

  // Select active mode values for display
  const activeInputText = mode === 'normalized' ? normInputText : naiveInputText;
  const activeInputAudio = mode === 'normalized' ? normInputAudio : naiveInputAudio;
  const activeTotalTokens = activeInputText + activeInputAudio + totalOutput;

  // Gemini 2.5 Flash Live API Rates
  // Input: Text $0.50 / 1M, Audio $3.00 / 1M
  // Output: Text $2.00 / 1M, Audio $12.00 / 1M
  const calcCost = (inTxt, inAud, outTxt, outAud) =>
    inTxt * (0.5 / 1000000) +
    inAud * (3.0 / 1000000) +
    outTxt * (2.0 / 1000000) +
    outAud * (12.0 / 1000000);

  const normCost = calcCost(normInputText, normInputAudio, outputText, outputAudio);
  const naiveCost = calcCost(naiveInputText, naiveInputAudio, outputText, outputAudio);
  const activeCost = mode === 'normalized' ? normCost : naiveCost;

  const inflationMultiplier = normTotalInput > 0 ? naiveTotalInput / normTotalInput : 1.0;

  const formatCost = (cost) => {
    if (!cost || cost === 0) return '$0.0000';
    if (cost < 0.0001) return `$${cost.toFixed(6)}`;
    if (cost < 0.01) return `$${cost.toFixed(4)}`;
    return `$${cost.toFixed(2)}`;
  };

  return (
    <div className="glass-panel" style={{ padding: '1rem', display: 'flex', flexDirection: 'column', gap: '0.75rem', overflow: 'hidden' }}>
      {/* Header & Mode Switcher */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
        <div style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-bright)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span>Token Telemetry</span>
          <span
            className="status-pill"
            style={{
              fontSize: '0.7rem',
              color: mode === 'normalized' ? 'var(--color-primary)' : '#ff9100',
              background: mode === 'normalized' ? 'rgba(0, 240, 255, 0.1)' : 'rgba(255, 145, 0, 0.12)',
              borderColor: mode === 'normalized' ? 'rgba(0, 240, 255, 0.2)' : 'rgba(255, 145, 0, 0.3)',
            }}
          >
            {formatNum(activeTotalTokens)} {mode === 'normalized' ? 'True Footprint' : 'Naive Summed'}
          </span>
        </div>

        {/* Interactive Mode Switcher Pills */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.35rem', background: 'rgba(0,0,0,0.35)', padding: '0.25rem', borderRadius: 'var(--border-radius-sm)', border: '1px solid var(--border-color)' }}>
          <button
            type="button"
            onClick={() => setMode('normalized')}
            style={{
              border: 'none',
              padding: '0.3rem 0.4rem',
              borderRadius: '4px',
              fontSize: '0.65rem',
              fontWeight: 600,
              cursor: 'pointer',
              transition: 'all 0.15s ease',
              background: mode === 'normalized' ? 'rgba(0, 240, 255, 0.18)' : 'transparent',
              color: mode === 'normalized' ? 'var(--color-primary)' : 'var(--text-muted)',
              boxShadow: mode === 'normalized' ? '0 0 8px rgba(0, 240, 255, 0.2)' : 'none',
            }}
          >
            ✓ Normalized (Watermark)
          </button>
          <button
            type="button"
            onClick={() => setMode('naive')}
            style={{
              border: 'none',
              padding: '0.3rem 0.4rem',
              borderRadius: '4px',
              fontSize: '0.65rem',
              fontWeight: 600,
              cursor: 'pointer',
              transition: 'all 0.15s ease',
              background: mode === 'naive' ? 'rgba(255, 145, 0, 0.2)' : 'transparent',
              color: mode === 'naive' ? '#ffab40' : 'var(--text-muted)',
              boxShadow: mode === 'naive' ? '0 0 8px rgba(255, 145, 0, 0.25)' : 'none',
            }}
          >
            ⚠️ Naive Event Sum
          </button>
        </div>

        <div style={{ fontSize: '0.62rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          Gemini 2.5 Flash Live Rates: $3.00/1M Aud In • $12.00/1M Aud Out
        </div>
      </div>

      {/* Main Ticker Grid */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem', fontSize: '0.75rem' }}>
        <div style={{ background: 'rgba(0,0,0,0.2)', padding: '0.5rem 0.75rem', borderRadius: 'var(--border-radius-sm)', border: '1px solid var(--border-color)', minWidth: 0 }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.62rem', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {mode === 'normalized' ? 'CONTEXT WINDOW (TXT / AUD)' : 'SUMMED PROMPT (TXT / AUD)'}
          </div>
          <div style={{ color: mode === 'normalized' ? 'var(--color-secondary)' : '#ffab40', fontWeight: 600, fontFamily: 'var(--font-mono)', marginTop: '0.2rem', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {formatNum(activeInputText)} / {formatNum(activeInputAudio)}
          </div>
        </div>

        <div style={{ background: 'rgba(0,0,0,0.2)', padding: '0.5rem 0.75rem', borderRadius: 'var(--border-radius-sm)', border: '1px solid var(--border-color)', minWidth: 0 }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.62rem', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            OUTPUT (TXT / AUD)
          </div>
          <div style={{ color: 'var(--color-primary)', fontWeight: 600, fontFamily: 'var(--font-mono)', marginTop: '0.2rem', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {formatNum(outputText)} / {formatNum(outputAudio)}
          </div>
        </div>
      </div>

      {/* Side-by-Side Comparison & Inflation Summary Bar */}
      <div
        style={{
          background: 'rgba(0,0,0,0.25)',
          padding: '0.45rem 0.65rem',
          borderRadius: 'var(--border-radius-sm)',
          border: '1px solid rgba(255,255,255,0.06)',
          display: 'flex',
          flexDirection: 'column',
          gap: '0.25rem',
          fontSize: '0.68rem',
          fontFamily: 'var(--font-mono)',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ color: 'var(--text-muted)' }}>Events Received: <strong style={{ color: 'var(--text-bright)' }}>{eventsCount}</strong></span>
          <span
            style={{
              color: inflationMultiplier > 1.05 ? '#ff9100' : '#00e676',
              fontWeight: 700,
              background: inflationMultiplier > 1.05 ? 'rgba(255, 145, 0, 0.12)' : 'rgba(0, 230, 118, 0.1)',
              padding: '0.1rem 0.4rem',
              borderRadius: '4px',
            }}
          >
            {inflationMultiplier.toFixed(2)}x Prompt Inflation
          </span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', color: 'var(--text-muted)', fontSize: '0.63rem' }}>
          <span>Watermark: <strong style={{ color: 'var(--color-primary)' }}>{formatNum(normTotalInput)} tok</strong> ({formatCost(normCost)})</span>
          <span>Naive Sum: <strong style={{ color: '#ffab40' }}>{formatNum(naiveTotalInput)} tok</strong> ({formatCost(naiveCost)})</span>
        </div>
      </div>

      {/* Below Ticker: Active Mode Session Cost Counter */}
      <div style={{ borderTop: '1px solid var(--border-color)', paddingTop: '0.6rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '0.75rem' }}>
        <span style={{ color: 'var(--text-muted)', fontSize: '0.7rem', fontWeight: 500 }}>
          {mode === 'normalized' ? 'Est. True Session Cost' : 'Est. Naive Summed Cost'}
        </span>
        <span
          title={`Exact Cost: $${activeCost.toFixed(8)}`}
          style={{
            color: mode === 'normalized' ? '#00e676' : '#ffab40',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            fontSize: '0.85rem',
            background: mode === 'normalized' ? 'rgba(0, 230, 118, 0.1)' : 'rgba(255, 145, 0, 0.12)',
            padding: '0.2rem 0.5rem',
            borderRadius: 'var(--border-radius-sm)',
            border: mode === 'normalized' ? '1px solid rgba(0, 230, 118, 0.2)' : '1px solid rgba(255, 145, 0, 0.3)',
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            maxWidth: '130px',
          }}
        >
          {formatCost(activeCost)}
        </span>
      </div>
    </div>
  );
}
