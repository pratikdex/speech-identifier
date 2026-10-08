'use client';

import { useEffect, useRef } from 'react';
import { useRecorder } from './RecorderContext';

export default function LiveWaveform() {
  const { isRecording, recorder } = useRecorder();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const audioContextRef = useRef<AudioContext | null>(null);
  const analyserRef = useRef<AnalyserNode | null>(null);
  const sourceRef = useRef<MediaStreamAudioSourceNode | null>(null);
  const rafIdRef = useRef<number | null>(null);

  // Resize canvas to match its display size for crisp drawing
  const resizeCanvasToDisplaySize = () => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const { clientWidth, clientHeight } = canvas;
    if (canvas.width !== clientWidth || canvas.height !== clientHeight) {
      canvas.width = clientWidth;
      canvas.height = clientHeight;
    }
  };

  const stopRendering = () => {
    if (rafIdRef.current) cancelAnimationFrame(rafIdRef.current);
    rafIdRef.current = null;
    const canvas = canvasRef.current;
    if (canvas) {
      const ctx = canvas.getContext('2d');
      if (ctx) {
        ctx.clearRect(0, 0, canvas.width, canvas.height);
      }
    }
  };

  const drawIdleLine = () => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    resizeCanvasToDisplaySize();
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const { width, height } = canvas;
    ctx.clearRect(0, 0, width, height);
    ctx.strokeStyle = '#000000';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(0, height / 2);
    ctx.lineTo(width, height / 2);
    ctx.stroke();
  };

  useEffect(() => {
    if (!isRecording) {
      stopRendering();
      drawIdleLine();
      try { void audioContextRef.current?.suspend(); } catch {}
      return;
    }

    let cancelled = false;

    const setup = async () => {
      if (!recorder) return;
      try {
        const stream = await recorder.getMediaStream();
        if (cancelled) return;

        if (!audioContextRef.current) {
          audioContextRef.current = new AudioContext();
        }
        const audioContext = audioContextRef.current;
        await audioContext.resume();

        if (!sourceRef.current) {
          sourceRef.current = new MediaStreamAudioSourceNode(audioContext, { mediaStream: stream });
        }
        if (!analyserRef.current) {
          analyserRef.current = new AnalyserNode(audioContext, { fftSize: 2048, smoothingTimeConstant: 0.8 });
          sourceRef.current.connect(analyserRef.current);
        }

        const analyser = analyserRef.current;
        const bufferLength = analyser.fftSize;
        const dataArray = new Uint8Array(bufferLength);

        const draw = () => {
          if (cancelled) return;
          rafIdRef.current = requestAnimationFrame(draw);
          const canvas = canvasRef.current;
          if (!canvas) return;
          const ctx = canvas.getContext('2d');
          if (!ctx) return;

          resizeCanvasToDisplaySize();
          analyser.getByteTimeDomainData(dataArray);

          const { width, height } = canvas;
          ctx.clearRect(0, 0, width, height);

          // Center line on white background
          ctx.strokeStyle = 'rgba(0, 0, 0, 0.1)';
          ctx.lineWidth = 1;
          ctx.beginPath();
          ctx.moveTo(0, height / 2);
          ctx.lineTo(width, height / 2);
          ctx.stroke();

          // Waveform
          ctx.lineWidth = 2;
          ctx.strokeStyle = '#000000';
          ctx.beginPath();

          const sliceWidth = width / bufferLength;
          let x = 0;
          for (let i = 0; i < bufferLength; i += 1) {
            const v = dataArray[i] / 128.0; // 0..2
            const y = (v * height) / 2;
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
            x += sliceWidth;
          }
          ctx.lineTo(width, height / 2);
          ctx.stroke();
        };

        stopRendering();
        draw();
      } catch {
        // ignore
      }
    };

    void setup();

    const handleResize = () => resizeCanvasToDisplaySize();
    window.addEventListener('resize', handleResize);
    return () => {
      cancelled = true;
      window.removeEventListener('resize', handleResize);
      stopRendering();
    };
  }, [isRecording, recorder]);

  // Ensure the idle line shows on initial mount and when resizing while idle
  useEffect(() => {
    if (!isRecording) drawIdleLine();
    const handleResize = () => { if (!isRecording) drawIdleLine(); };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [isRecording]);

  return (
    <div className="w-full h-[120px] rounded-md overflow-hidden bg-white border border-black">
      <canvas ref={canvasRef} className="w-full h-full" />
    </div>
  );
}

