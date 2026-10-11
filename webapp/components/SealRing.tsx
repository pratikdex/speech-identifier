'use client';

import { useEffect, useRef } from 'react';
import { DictationPhase } from '@/hooks/useDictation';

const SIZE = 128;
const BARS = 48;
const RADIUS = 30;
const ACCENT = '#6366f1';

// SealRing is the record button: a circular waveform around an ink disc.
export function SealRing({ phase, analyser, onClick }: { phase: DictationPhase; analyser: AnalyserNode | null; onClick(): void }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext('2d');
    if (!canvas || !ctx) return;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = canvas.height = SIZE * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const data = new Uint8Array(analyser?.frequencyBinCount ?? 0);
    const c = SIZE / 2;
    const level = new Array(BARS).fill(0); // smoothed amplitude per point
    let raf = 0;

    const draw = (t: number) => {
      ctx.clearRect(0, 0, SIZE, SIZE);
      analyser?.getByteFrequencyData(data);
      // Loudness drives the size; a rotating sine gives the wave shape. Using the raw
      // spectrum per point put the loud bass bins on one side and made the ring lopsided.
      let sum = 0;
      for (let k = 1; k < 32 && k < data.length; k++) sum += data[k];
      const loud = analyser ? sum / 31 / 255 : 0.08;
      for (let i = 0; i < BARS; i++) {
        const a = (i / BARS) * Math.PI * 2;
        const target = loud * 22 * (0.55 + 0.45 * Math.sin(3 * a + t / 250));
        level[i] += (target - level[i]) * 0.2;
      }
      const pt = (i: number) => {
        const a = ((i % BARS) / BARS) * Math.PI * 2;
        const r = RADIUS + 4 + level[i % BARS];
        return [c + Math.cos(a) * r, c + Math.sin(a) * r];
      };
      ctx.strokeStyle = ACCENT;
      ctx.fillStyle = ACCENT + '1a';
      ctx.lineWidth = 2;
      ctx.beginPath();
      // Quadratic curves through midpoints give a smooth closed blob.
      const [sx, sy] = pt(0).map((v, j) => (v + pt(1)[j]) / 2);
      ctx.moveTo(sx, sy);
      for (let i = 1; i <= BARS; i++) {
        const [x, y] = pt(i);
        const [nx, ny] = pt(i + 1);
        ctx.quadraticCurveTo(x, y, (x + nx) / 2, (y + ny) / 2);
      }
      ctx.closePath();
      ctx.fill();
      ctx.stroke();

      ctx.fillStyle = ACCENT;
      ctx.beginPath();
      ctx.arc(c, c, RADIUS * 0.8, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = ctx.strokeStyle = '#fff';
      if (phase === DictationPhase.Listening) ctx.fillRect(c - 6, c - 6, 12, 12); // stop
      else if (phase === DictationPhase.Idle) {
        ctx.lineWidth = 2;
        ctx.lineCap = 'round';
        ctx.beginPath();
        ctx.roundRect(c - 4, c - 11, 8, 14, 4); // mic head
        ctx.fill();
        ctx.beginPath();
        ctx.arc(c, c - 2, 9, 0, Math.PI); // cradle
        ctx.moveTo(c, c + 7);
        ctx.lineTo(c, c + 12);
        ctx.stroke();
      } else {
        ctx.beginPath();
        ctx.arc(c, c, 8, t / 200, t / 200 + Math.PI * 1.5); // spinner
        ctx.lineWidth = 2;
        ctx.stroke();
      }
      raf = requestAnimationFrame(draw);
    };
    raf = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(raf);
  }, [phase, analyser]);

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={phase === DictationPhase.Listening ? 'Stop dictation' : 'Start dictation'}
      className="rounded-full focus-visible:outline-2 focus-visible:outline-ink"
    >
      <canvas ref={canvasRef} style={{ width: SIZE, height: SIZE }} aria-hidden />
    </button>
  );
}
