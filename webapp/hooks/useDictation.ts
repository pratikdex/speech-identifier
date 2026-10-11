'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Recorder } from '@/recorder/recorder';
import {
  startDictation,
  DictationMode,
  ErrorCode,
  ServerEventType,
  type Utterance,
} from '@/lib/dictation';
import { Lang } from '@/lib/languages';

export enum DictationPhase {
  Idle = 'idle',
  Connecting = 'connecting',
  Listening = 'listening',
  Stopping = 'stopping',
}

export const ERROR_MESSAGES: Record<string, string> = {
  [ErrorCode.MicDenied]: 'Microphone access is needed to dictate.',
  [ErrorCode.UnsupportedLanguage]: "This language isn't supported for that option yet.",
  [ErrorCode.ConnectionLost]: 'Connection lost. Please try again.',
  [ErrorCode.ProviderUnavailable]: 'Speech service is unavailable. Please try again.',
  [ErrorCode.BadConfig]: 'Invalid dictation configuration.',
};
export const DEFAULT_ERROR_MESSAGE = "Couldn't reach the speech service. Please try again.";

export function useDictation() {
  const [phase, setPhase] = useState<DictationPhase>(DictationPhase.Idle);
  const [lines, setLines] = useState<Utterance[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<DictationMode>(DictationMode.Transcribe);
  const [target, setTarget] = useState<Lang>(Lang.English);
  const [analyser, setAnalyser] = useState<AnalyserNode | null>(null);

  const recorderRef = useRef<Recorder | null>(null);
  const sessionRef = useRef<Awaited<ReturnType<typeof startDictation>> | null>(null);

  useEffect(() => {
    return () => {
      sessionRef.current?.close();
      void recorderRef.current?.destroy();
      recorderRef.current = null;
    };
  }, []);

  // Mic analyser for the waveform while listening.
  useEffect(() => {
    const recorder = recorderRef.current;
    if (phase !== DictationPhase.Listening || !recorder) return;
    let ctx: AudioContext | null = null;
    let cancelled = false;
    void recorder.getMediaStream().then((stream) => {
      if (cancelled) return;
      ctx = new AudioContext();
      const node = ctx.createAnalyser();
      node.fftSize = 256;
      ctx.createMediaStreamSource(stream).connect(node);
      setAnalyser(node);
    });
    return () => {
      cancelled = true;
      setAnalyser(null);
      void ctx?.close();
    };
  }, [phase]);

  const start = useCallback(async () => {
    // A fresh recorder per session: destroy() releases the mic and the old one can't be reused.
    const recorder = new Recorder();
    recorderRef.current = recorder;
    setError(null);
    setLines([]);
    setPhase(DictationPhase.Connecting);
    try {
      sessionRef.current = await startDictation(recorder, { mode, target }, {
        onEvent(e) {
          if (e.type === ServerEventType.Utterance) {
            setLines((prev) => {
              const i = prev.findIndex((l) => l.id === e.id && !l.final);
              return i >= 0 ? prev.map((l, j) => (j === i ? e : l)) : [...prev, e];
            });
          } else if (e.type === ServerEventType.Error) {
            setError(ERROR_MESSAGES[e.code] ?? DEFAULT_ERROR_MESSAGE);
          }
        },
        onClose() {
          sessionRef.current = null;
          void recorder.destroy();
          setPhase(DictationPhase.Idle);
        },
      });
      setPhase(DictationPhase.Listening);
    } catch (err) {
      setError(err instanceof Error && err.name === 'NotAllowedError' ? ERROR_MESSAGES[ErrorCode.MicDenied] : DEFAULT_ERROR_MESSAGE);
      void recorder.destroy();
      setPhase(DictationPhase.Idle);
    }
  }, [mode, target]);

  const toggle = useCallback(() => {
    if (phase === DictationPhase.Idle) void start();
    else if (phase === DictationPhase.Listening) {
      setPhase(DictationPhase.Stopping);
      sessionRef.current?.stop();
    }
  }, [phase, start]);

  return { phase, lines, error, mode, setMode, target, setTarget, analyser, toggle };
}
