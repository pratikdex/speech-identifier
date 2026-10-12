import type { Recorder } from '@/recorder/recorder';
import type { Lang } from './languages';

export enum DictationMode {
  Transcribe = 'transcribe',
  Translate = 'translate',
}

// Mirrors the wire protocol in server/internal/server/protocol.go; keep in sync.
export enum ServerEventType {
  Utterance = 'utterance',
  Error = 'error',
  Done = 'done',
}

export enum ErrorCode {
  MicDenied = 'mic_denied',
  UnsupportedLanguage = 'unsupported_language',
  ConnectionLost = 'connection_lost',
  ProviderUnavailable = 'provider_unavailable',
  BadConfig = 'bad_config',
}

export type Utterance = { id: string; text: string; lang: string; final: boolean; translation?: string; confidence?: number };

export type ServerEvent =
  | ({ type: ServerEventType.Utterance | 'utterance' } & Utterance)
  | { type: ServerEventType.Error | 'error'; code: ErrorCode | string }
  | { type: ServerEventType.Done | 'done' };

type Handlers = { onEvent(e: ServerEvent): void; onClose(): void };

// startDictation opens the WebSocket, sends the config, then streams mic audio.
export async function startDictation(
  recorder: Recorder,
  settings: { mode: DictationMode; target: Lang },
  { onEvent, onClose }: Handlers,
) {
  const sampleRate = await recorder.getAudioSampleRate();
  const url = new URL(process.env.NEXT_PUBLIC_DICTATION_ENDPOINT ?? 'ws://localhost:8081/ws');
  url.searchParams.set('sample_rate', String(sampleRate));
  const ws = new WebSocket(url);

  const sendAudio = (e: Event) => {
    const data = (e as BlobEvent).data;
    if (!data?.size || ws.readyState !== WebSocket.OPEN) return;
    // Drop the WAV header the encoder prepends to each chunk.
    let blob = new Blob([data], { type: 'audio/wav' });
    if (blob.size > sampleRate) blob = blob.slice(blob.size % sampleRate);
    ws.send(blob);
  };

  ws.onmessage = (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch {
      // ignore non-JSON frames
    }
  };
  ws.onclose = () => {
    void recorder.removeEventListener('dataavailable', sendAudio);
    onClose();
  };

  await new Promise<void>((resolve, reject) => {
    ws.onopen = () => resolve();
    ws.onerror = () => reject(new Error('connection failed'));
  });

  ws.send(JSON.stringify({ type: 'config', ...settings }));
  try {
    await recorder.addEventListener('dataavailable', sendAudio);
    await recorder.startMediaRecorder(500);
  } catch (err) {
    ws.close();
    throw err;
  }

  return {
    stop() {
      void recorder.removeEventListener('dataavailable', sendAudio);
      void recorder.stopMediaRecorder();
      if (ws.readyState === WebSocket.OPEN) ws.send('STOP');
      setTimeout(() => ws.close(), 5000); // don't hang in "stopping" if the server never replies
    },
    close() {
      void recorder.stopMediaRecorder();
      ws.close();
    },
  };
}
