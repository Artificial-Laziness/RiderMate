import { describe, it, expect } from 'vitest';

describe('Audio PCM Conversion Unit Tests', () => {
  it('converts a mock base64 PCM16 chunk to float32 samples accurately', () => {
    // Standard 4 bytes of silence representing two 16-bit integer samples
    const base64Data = "AAAAAA==";
    
    const binaryString = atob(base64Data);
    const len = binaryString.length;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }
    const pcm16 = new Int16Array(bytes.buffer);
    
    expect(pcm16.length).toBe(2);
    expect(pcm16[0]).toBe(0);
    expect(pcm16[1]).toBe(0);

    const float32Samples = new Float32Array(pcm16.length);
    for (let i = 0; i < pcm16.length; i++) {
      float32Samples[i] = pcm16[i] / 32768.0;
    }

    expect(float32Samples[0]).toBeCloseTo(0.0, 5);
    expect(float32Samples[1]).toBeCloseTo(0.0, 5);
  });

  it('verifies peak volume clipping safeguards', () => {
    const clamp = (val) => Math.max(-1, Math.min(1, val));
    const toInt16 = (val) => {
      const s = clamp(val);
      return s < 0 ? s * 0x8000 : s * 0x7fff;
    };

    expect(toInt16(0.0)).toBe(0);
    expect(toInt16(1.0)).toBe(32767);
    expect(toInt16(-1.0)).toBe(-32768);
    expect(toInt16(1.5)).toBe(32767); // Clipped cleanly
    expect(toInt16(-2.0)).toBe(-32768); // Clipped cleanly
  });
});
