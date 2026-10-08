'use client';
// Single-class Recorder using MediaRecorder with WAV via polyfill.
// No Sentry, no toast, no device-change handling.
// NOTE: Avoid importing worker-dependent libs at module scope to keep SSR safe.
// We'll dynamically import them inside methods when running on the client.

export class Recorder {
  private recorder?: MediaRecorder;
  private eventListeners: Record<string, EventListener[]> = {};

  private stream?: MediaStream;
  private originalStream?: MediaStream;
  private audioContext?: AudioContext;
  private sourceNode?: MediaStreamAudioSourceNode;
  private destinationNode?: MediaStreamAudioDestinationNode;

  private async setupStreams(options?: MediaStreamConstraints): Promise<void> {
    if (this.stream) return;
    const constraints = options ?? { audio: true, video: false };
    const mic = await navigator.mediaDevices.getUserMedia(constraints);
    this.originalStream = mic;

    this.audioContext = new AudioContext();
    this.sourceNode = new MediaStreamAudioSourceNode(this.audioContext, { mediaStream: mic });
    this.destinationNode = new MediaStreamAudioDestinationNode(this.audioContext, { channelCount: 1 });
    this.sourceNode.connect(this.destinationNode);
    this.stream = this.destinationNode.stream;

    try {
      const track = mic.getAudioTracks()[0];
      await track.applyConstraints?.({ channelCount: 1 });
    } catch {}
  }

  private destroyStream(): void {
    if (this.stream) {
      this.stream.getTracks().forEach((t) => t.stop());
      this.stream = undefined;
    }
    try { this.sourceNode?.disconnect(); } catch {}
    this.sourceNode = undefined;
    this.destinationNode = undefined;
    if (this.audioContext) {
      void this.audioContext.close();
      this.audioContext = undefined;
    }
    this.originalStream = undefined;
  }

  public getOriginalStream(): MediaStream | undefined {
    return this.originalStream;
  }

  getMediaStream = async (): Promise<MediaStream> => {
    await this.setupStreams();
    return this.stream!;
  };

  getAudioSampleRate = async (): Promise<number> => {
    if (this.audioContext) return this.audioContext.sampleRate;
    const temp = new AudioContext();
    const rate = temp.sampleRate;
    await temp.close();
    return rate;
  };

  getExpectedBlobSizeSync = async (): Promise<number> => {
    const sampleRate = await this.getAudioSampleRate();
    const isFirefox = navigator.userAgent.toLowerCase().includes('firefox');
    return isFirefox ? sampleRate * 2 : sampleRate;
  };

  getMediaRecorder = async (): Promise<MediaRecorder> => {
    if (this.recorder) return this.recorder;
    await this.setupStreams();

    const original = this.getOriginalStream();
    let recordStream: MediaStream = this.stream!;
    try {
      const origChannels = original?.getAudioTracks()[0]?.getSettings().channelCount;
      if (origChannels === 1 && original) {
        await original.getAudioTracks()[0]?.applyConstraints?.({ channelCount: 1 });
        recordStream = original;
      }
    } catch {}

    // Dynamically import the polyfill and encoder on the client where workers exist
    try {
      if (typeof window !== 'undefined') {
        let MediaRecorderConstructor: typeof MediaRecorder = (window as unknown as { MediaRecorder: typeof MediaRecorder }).MediaRecorder;
        if (typeof Worker !== 'undefined') {
          const { MediaRecorder: PolyfillMediaRecorder, register } = await import('extendable-media-recorder');
          const { connect } = await import('extendable-media-recorder-wav-encoder');
          try { await register(await connect()); } catch {}
          MediaRecorderConstructor = PolyfillMediaRecorder as unknown as typeof MediaRecorder;
        }
        this.recorder = new MediaRecorderConstructor(recordStream, { mimeType: 'audio/wav' });
      }
    } catch {}
    if (!this.recorder) this.recorder = new MediaRecorder(recordStream);

    Object.entries(this.eventListeners).forEach(([type, listeners]) => {
      listeners.forEach((listener) => this.recorder?.addEventListener(type, listener));
    });
    return this.recorder;
  };

  startMediaRecorder = async (chunkInterval = 500, failureCallback?: () => void) => {
    console.log("startMediaRecorder", chunkInterval);
    try {
      const recorder = await this.getMediaRecorder();
      if (recorder.state !== 'inactive') throw new Error('Recorder is already active');
      recorder.start(chunkInterval);
      return recorder;
    } catch (e) {
      console.log("error", e);  
      if (failureCallback) failureCallback();
      // eslint-disable-next-line no-console
      console.error('Error starting media recorder', e);
      throw e;
    }
  };

  stopMediaRecorder = async () => {
    try {
      const recorder = await this.getMediaRecorder();
      recorder.stop();
      return recorder;
    } catch (e) {
      // eslint-disable-next-line no-console
      console.error('Error stopping media recorder', e);
    }
  };

  resetRecorder = async () => {
    try {
      const stream = await this.getMediaStream();
      const recorder = await this.getMediaRecorder();
      stream.getAudioTracks().forEach((track) => (track.enabled = true));
      if (recorder.state === 'inactive') return recorder;
      if (recorder.state === 'paused') recorder.resume();
      Object.entries(this.eventListeners).forEach(([type, listeners]) =>
        listeners.forEach((listener) => recorder.removeEventListener(type, listener))
      );
      recorder.stop();
      return recorder;
    } catch (e) {
      // eslint-disable-next-line no-console
      console.error('Error resetting media recorder', e);
    }
  };

  addEventListener = async (type: string, listener: EventListener) => {
    const recorder = await this.getMediaRecorder();
    if (!this.eventListeners[type]) this.eventListeners[type] = [];
    this.eventListeners[type]?.push(listener);
    recorder.addEventListener(type, listener);
  };

  removeEventListener = async (type: string, listener: EventListener) => {
    const recorder = await this.getMediaRecorder();
    const listeners = this.eventListeners[type];
    if (!listeners) return;
    this.eventListeners[type] = listeners.filter((l) => l !== listener);
    recorder.removeEventListener(type, listener);
  };

  get state() {
    return this.recorder?.state;
  }

  destroy = async () => {
    await this.resetRecorder();
    this.destroyStream();
  };
}

