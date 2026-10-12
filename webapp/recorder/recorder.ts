'use client';
// Recorder wraps MediaRecorder with a WAV encoder polyfill. The polyfill is
// imported inside methods because it needs workers, which SSR doesn't have.

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
    this.originalStream?.getTracks().forEach((t) => t.stop()); // the real mic; stopping only the copy leaves the tab's mic indicator on
    this.originalStream = undefined;
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

  getMediaRecorder = async (): Promise<MediaRecorder> => {
    if (this.recorder) return this.recorder;
    await this.setupStreams();

    // A mono mic can be recorded directly; otherwise record the mono mixdown.
    const mono = this.originalStream?.getAudioTracks()[0]?.getSettings().channelCount === 1;
    const recordStream = mono ? this.originalStream! : this.stream!;

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

  startMediaRecorder = async (chunkInterval = 500) => {
    const recorder = await this.getMediaRecorder();
    if (recorder.state !== 'inactive') throw new Error('Recorder is already active');
    recorder.start(chunkInterval);
    return recorder;
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
      const recorder = await this.getMediaRecorder();
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

  destroy = async () => {
    if (!this.stream) return; // already released; resetRecorder would reopen the mic
    await this.resetRecorder();
    this.destroyStream();
  };
}

