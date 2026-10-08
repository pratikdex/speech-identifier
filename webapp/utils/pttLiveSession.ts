'use client';

import { Recorder } from '@/recorder/recorder';
import { nanoid } from 'nanoid';
export type PttSession = {
  startSession: () => Promise<void>;
  endSession: () => Promise<void>;
  destroy: () => void;
};

export async function getPTTLiveRecognizer(language: string, recorder: Recorder = new Recorder()): Promise<PttSession> {
  let webSocket: WebSocket | null = null;

  const onStartEvent = () => {
    try { webSocket?.send('START'); } catch {}
  };

  const onMessage = async (e: Event) => {
    const event = e as BlobEvent;
    if (!navigator.onLine || !webSocket || webSocket.readyState !== WebSocket.OPEN) return;
    if (!event.data || (event.data as Blob).size <= 0) return;
    const audioSampleRate = await recorder.getAudioSampleRate();
    let blob = new Blob([event.data], { type: 'audio/wav' });
    if (blob.size > audioSampleRate) {
      const startPos = blob.size % audioSampleRate;
      blob = blob.slice(startPos, blob.size, 'audio/wav');
    }
    webSocket?.send(blob)
  };
  const onMessageEvent = (e: Event) => void onMessage(e);

  const onStopEvent = async () => {
    webSocket?.send('STOP');
    webSocket?.close();
    await recorder.removeEventListener('dataavailable', onMessageEvent);
    await recorder.removeEventListener('stop', onStopEvent as unknown as EventListener);
    await recorder.removeEventListener('start', onStartEvent as unknown as EventListener);
  };


  const onWebSocketMessage = (e: MessageEvent<string>) => {
    if (typeof e.data === 'string' && (e.data.includes('READY') || e.data.includes('KEEP'))) return;
    window.dispatchEvent(new CustomEvent('ptt:recognized', { detail: e.data }));
  };

  const startSession = async () => {
    await recorder.addEventListener('dataavailable', onMessageEvent);
    await recorder.addEventListener('start', onStartEvent as unknown as EventListener);
    await recorder.addEventListener('stop', onStopEvent as unknown as EventListener);

    if (!webSocket || webSocket.readyState !== WebSocket.OPEN) {
      const url = new URL(process.env.NEXT_PUBLIC_DICTATION_ENDPOINT || ``);
      const audioSampleRate = await recorder.getAudioSampleRate();
      const langParam = language;
      const searchParams = new URLSearchParams({
        language: langParam,
        mode: 'batch',
        request_id: nanoid(),
        sample_rate: audioSampleRate.toString(),
      });
      url.search = searchParams.toString();
      webSocket = new WebSocket(url);
      webSocket.onmessage = onWebSocketMessage;
      webSocket.onopen = () => {
        void recorder.startMediaRecorder(500);
      };
    } else void recorder.startMediaRecorder(500);
  };

  const endSession = async () => {
    await recorder.stopMediaRecorder();
  };

  const destroy = () => {
    webSocket?.close();
  };

  return { startSession, endSession, destroy };
}

// Adapter used by the toggle button (like pttSession): returns a handle with stop()/etc
export async function startPttLiveSession(language: string) {
  const session = await getPTTLiveRecognizer(language);
  await session.startSession();
  return session;
}
