import { h } from 'preact';
import { useEffect, useRef } from 'preact/hooks';

export function Visualizer({ audioManager, isRecording, isSpeaking }) {
  const canvasRef = useRef(null);
  const animationRef = useRef(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    let width = canvas.width = canvas.offsetWidth * window.devicePixelRatio;
    let height = canvas.height = canvas.offsetHeight * window.devicePixelRatio;
    ctx.scale(window.devicePixelRatio, window.devicePixelRatio);

    const resizeObserver = new ResizeObserver((entries) => {
      for (let entry of entries) {
        if (canvas) {
          width = canvas.width = entry.contentRect.width * window.devicePixelRatio;
          height = canvas.height = entry.contentRect.height * window.devicePixelRatio;
          const context = canvas.getContext('2d');
          context.scale(window.devicePixelRatio, window.devicePixelRatio);
        }
      }
    });
    resizeObserver.observe(canvas.parentElement);

    const dataArrayMic = new Uint8Array(128);
    const dataArraySpeaker = new Uint8Array(128);

    let idlePhase = 0;

    const draw = () => {
      animationRef.current = requestAnimationFrame(draw);

      const w = width / window.devicePixelRatio;
      const h = height / window.devicePixelRatio;

      ctx.fillStyle = 'rgba(10, 11, 16, 0.2)';
      ctx.fillRect(0, 0, w, h);

      let micActive = false;
      let speakerActive = false;

      if (audioManager && audioManager.micAnalyser && isRecording) {
        audioManager.micAnalyser.getByteFrequencyData(dataArrayMic);
        micActive = dataArrayMic.some(val => val > 0);
      }

      if (audioManager && audioManager.speakerAnalyser && isSpeaking) {
        audioManager.speakerAnalyser.getByteFrequencyData(dataArraySpeaker);
        speakerActive = dataArraySpeaker.some(val => val > 0);
      }

      if (micActive || speakerActive) {
        if (speakerActive) {
          ctx.strokeStyle = '#00f0ff';
          ctx.lineWidth = 3;
          ctx.shadowColor = 'rgba(0, 240, 255, 0.5)';
          ctx.shadowBlur = 15;
          ctx.beginPath();

          const sliceWidth = w / dataArraySpeaker.length;
          let x = 0;

          for (let i = 0; i < dataArraySpeaker.length; i++) {
            const v = dataArraySpeaker[i] / 255.0;
            const amplitude = v * (h * 0.45);
            const y = (h / 2) + (i % 2 === 0 ? amplitude : -amplitude);

            if (i === 0) {
              ctx.moveTo(x, y);
            } else {
              ctx.lineTo(x, y);
            }
            x += sliceWidth;
          }
          ctx.stroke();
        }

        if (micActive) {
          ctx.strokeStyle = '#00e676';
          ctx.lineWidth = 3;
          ctx.shadowColor = 'rgba(0, 230, 118, 0.6)';
          ctx.shadowBlur = 15;
          ctx.beginPath();

          const sliceWidth = w / dataArrayMic.length;
          let x = 0;

          for (let i = 0; i < dataArrayMic.length; i++) {
            const v = dataArrayMic[i] / 255.0;
            const amplitude = v * (h * 0.4);
            const y = (h / 2) + Math.sin(i * 0.1) * amplitude;

            if (i === 0) {
              ctx.moveTo(x, y);
            } else {
              ctx.lineTo(x, y);
            }
            x += sliceWidth;
          }
          ctx.stroke();
        }
      } else {
        ctx.shadowBlur = 0;
        idlePhase += 0.04;
        ctx.lineWidth = 2;
        
        for (let waveIndex = 0; waveIndex < 3; waveIndex++) {
          const opacity = 0.15 - waveIndex * 0.04;
          ctx.strokeStyle = waveIndex === 0 ? `rgba(0, 230, 118, ${opacity})` : (waveIndex === 1 ? `rgba(0, 240, 255, ${opacity})` : `rgba(255, 255, 255, ${opacity})`);
          
          ctx.beginPath();
          for (let x = 0; x < w; x++) {
            const freqFactor = 0.007 + waveIndex * 0.003;
            const ampFactor = 12 - waveIndex * 3;
            const speedFactor = idlePhase * (1 - waveIndex * 0.2);
            const y = (h / 2) + Math.sin(x * freqFactor + speedFactor) * ampFactor;
            
            if (x === 0) {
              ctx.moveTo(x, y);
            } else {
              ctx.lineTo(x, y);
            }
          }
          ctx.stroke();
        }
      }

      ctx.shadowBlur = 0;
    };

    draw();

    return () => {
      cancelAnimationFrame(animationRef.current);
      resizeObserver.disconnect();
    };
  }, [audioManager, isRecording, isSpeaking]);

  return (
    <div className="canvas-container">
      <canvas ref={canvasRef} />
    </div>
  );
}
