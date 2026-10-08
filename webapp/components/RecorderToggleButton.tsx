'use client';

import { useEffect, useRef } from 'react';
import { useRecorder } from './RecorderContext';
import { startPttLiveSession } from '@/utils/pttLiveSession';

export default function RecorderToggleButton() {
  const { isRecording, stopRecording, startRecording, language } = useRecorder()
  const sessionRef = useRef<Awaited<ReturnType<typeof startPttLiveSession>> | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function startFlow() {
      sessionRef.current = await startPttLiveSession(language);
    }

    async function stopFlow() {
      if (sessionRef.current) {
        await sessionRef.current.endSession();
        sessionRef.current.destroy();
        sessionRef.current = null;
      }
    }

    if (isRecording && !cancelled) {
      void startFlow();
    } else if (!isRecording && sessionRef.current) {
      void stopFlow();
    }

    return () => {
      cancelled = true;
      if (sessionRef.current) {
        void sessionRef.current.endSession();
        sessionRef.current.destroy();
        sessionRef.current = null;
      }
    };
  }, [isRecording, language]);

  return (
    <div className="flex items-center gap-2">
      {isRecording && (
        <span className="text-sm text-red-700">Recording...</span>
      )}
      <button
        type="button"
        onClick={isRecording ? stopRecording : startRecording}
        className={
          `px-3 py-1.5 rounded-md text-sm font-medium cursor-pointer ${
            isRecording
              ? 'bg-red-100 text-red-900 hover:ring-1 hover:ring-inset hover:ring-red-900'
              : 'bg-grey-900 text-white enabled:hover:bg-indigo-600 border-0'
          }`
        }
        aria-pressed={isRecording}
      >
        {isRecording ? 'Stop' : 'Start'} Recording
      </button>
    </div>
  );
}

