/**
 * Web Audio API Manager
 * Handles 16kHz mono audio recording (mic) and crackle-free scheduled audio playback.
 * Provides real-time AnalyserNodes for mic and speaker visualizers.
 */

export class AudioManager {
  constructor() {
    this.audioCtx = null;
    this.micStream = null;
    this.processorNode = null;
    this.sourceNode = null;
    this.activeSources = [];
    this.nextPlayTime = 0;
    this.sampleRate = 16000;
    
    // Analyser nodes for live waveform rendering
    this.micAnalyser = null;
    this.speakerAnalyser = null;
  }

  /**
   * Initialize Audio Context. Must be called from a user gesture handler.
   */
  async init() {
    if (this.audioCtx) {
      if (this.audioCtx.state === 'suspended') {
        await this.audioCtx.resume();
      }
      return;
    }

    // Force context to 16kHz mono to perform automatic browser resampling on both inputs and outputs
    this.audioCtx = new (window.AudioContext || window.webkitAudioContext)({
      sampleRate: this.sampleRate,
    });

    if (this.audioCtx.state === 'suspended') {
      await this.audioCtx.resume();
    }
    
    this.nextPlayTime = this.audioCtx.currentTime;

    // Initialize speaker analyzer
    this.speakerAnalyser = this.audioCtx.createAnalyser();
    this.speakerAnalyser.fftSize = 256;
    this.speakerAnalyser.connect(this.audioCtx.destination);
  }

  /**
   * Start recording from user's microphone.
   * @param {function} onAudioChunk - Callback triggered with ArrayBuffer (Int16 PCM)
   */
  async startRecording(onAudioChunk) {
    await this.init();

    this.micStream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        sampleRate: this.sampleRate,
        echoCancellation: true,
        noiseSuppression: true,
      },
    });

    this.sourceNode = this.audioCtx.createMediaStreamSource(this.micStream);
    
    // Create and attach mic analyzer
    this.micAnalyser = this.audioCtx.createAnalyser();
    this.micAnalyser.fftSize = 256;
    this.sourceNode.connect(this.micAnalyser);

    // ScriptProcessor is widely supported and very reliable for raw sample access
    this.processorNode = this.audioCtx.createScriptProcessor(2048, 1, 1);

    this.processorNode.onaudioprocess = (e) => {
      if (!onAudioChunk) return;

      const float32Samples = e.inputBuffer.getChannelData(0);
      const pcm16Buffer = new Int16Array(float32Samples.length);

      for (let i = 0; i < float32Samples.length; i++) {
        const s = Math.max(-1, Math.min(1, float32Samples[i]));
        pcm16Buffer[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
      }

      onAudioChunk(pcm16Buffer.buffer);
    };

    this.sourceNode.connect(this.processorNode);
    // Connect to destination to run processing loop, but output silence (no howl)
    this.processorNode.connect(this.audioCtx.destination);
  }

  /**
   * Stop microphone recording.
   */
  stopRecording() {
    if (this.processorNode) {
      this.processorNode.disconnect();
      this.processorNode.onaudioprocess = null;
      this.processorNode = null;
    }
    if (this.sourceNode) {
      this.sourceNode.disconnect();
      this.sourceNode = null;
    }
    if (this.micStream) {
      this.micStream.getTracks().forEach(track => track.stop());
      this.micStream = null;
    }
    this.micAnalyser = null;
  }

  /**
   * Queue Base64 PCM audio data for play.
   * Schedules chunks consecutively to avoid crackling and gap click noises.
   * @param {string} base64Data - Base64 encoded signed 16-bit PCM
   */
  playPCMChunk(base64Data) {
    if (!this.audioCtx || !this.speakerAnalyser) return;

    // Convert Base64 string to Int16Array
    const binaryString = window.atob(base64Data);
    const len = binaryString.length;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }
    const pcm16 = new Int16Array(bytes.buffer);

    // Convert Int16PCM to Float32 audio samples
    const float32Samples = new Float32Array(pcm16.length);
    for (let i = 0; i < pcm16.length; i++) {
      float32Samples[i] = pcm16[i] / 32768.0;
    }

    // Create an AudioBuffer and populate it
    const audioBuffer = this.audioCtx.createBuffer(1, float32Samples.length, this.sampleRate);
    audioBuffer.getChannelData(0).set(float32Samples);

    const source = this.audioCtx.createBufferSource();
    source.buffer = audioBuffer;
    
    // Connect source to the speaker analyzer (which outputs to speakers)
    source.connect(this.speakerAnalyser);

    const now = this.audioCtx.currentTime;
    // If we've lagged behind, align the play scheduling head close to the present
    if (this.nextPlayTime < now) {
      this.nextPlayTime = now + 0.05; // 50ms leeway to prevent underrun gaps
    }

    source.start(this.nextPlayTime);
    
    // Schedule the next chunk exactly at the end of this one
    this.nextPlayTime += audioBuffer.duration;

    // Keep track of the active node so we can immediately abort it on interrupt
    source.onended = () => {
      const idx = this.activeSources.indexOf(source);
      if (idx > -1) {
        this.activeSources.splice(idx, 1);
      }
    };
    this.activeSources.push(source);
  }

  /**
   * Flush/Cancel all scheduled playback immediately (for interruptions).
   */
  clearPlaybackQueue() {
    this.activeSources.forEach((source) => {
      try {
        source.stop();
      } catch {
        // Source might have already finished playing
      }
    });
    this.activeSources = [];
    if (this.audioCtx) {
      this.nextPlayTime = this.audioCtx.currentTime;
    }
  }

  /**
   * Close and clean up all audio resources.
   */
  async close() {
    this.stopRecording();
    this.clearPlaybackQueue();
    if (this.audioCtx) {
      await this.audioCtx.close();
      this.audioCtx = null;
    }
  }
}
