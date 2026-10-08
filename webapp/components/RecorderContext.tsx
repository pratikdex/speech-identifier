'use client';

import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react';
import { Recorder } from '@/recorder/recorder';

type RecorderContextValue = {
  isRecording: boolean;
  startRecording: () => void;
  stopRecording: () => void;
  language: string;
  setLanguage: (code: string) => void;
  recorder: Recorder | null;
};

const RecorderContext = createContext<RecorderContextValue | undefined>(undefined);

type RecorderProviderProps = {
  children: React.ReactNode;
};

export default function RecorderProvider({ children }: RecorderProviderProps) {
  const [isRecording, setIsRecording] = useState(false);
  const [language, setLanguage] = useState<string>('en');
  const recorderRef = useRef<Recorder | null>(null);

  if (recorderRef.current === null && typeof window !== 'undefined') {
    recorderRef.current = new Recorder();
  }

  const startRecording = useCallback(() => {
    setIsRecording(true);
  }, []);

  const stopRecording = useCallback(() => {
    setIsRecording(false);
  }, []);

  const value = useMemo(
    () => ({ isRecording, startRecording, stopRecording, language, setLanguage, recorder: recorderRef.current }),
    [isRecording, startRecording, stopRecording, language]
  );

  return <RecorderContext.Provider value={value}>{children}</RecorderContext.Provider>;
}

export function useRecorder(): RecorderContextValue {
  const ctx = useContext(RecorderContext);
  if (!ctx) {
    throw new Error('useRecorder must be used within a RecorderProvider');
  }
  return ctx;
}

