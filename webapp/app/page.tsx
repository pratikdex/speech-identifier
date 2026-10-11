'use client';

import { useDictation, DictationPhase } from '@/hooks/useDictation';
import { DictationMode } from '@/lib/dictation';
import { LANGUAGE_NAMES, languageLabel, type Lang } from '@/lib/languages';
import { SealRing } from '@/components/SealRing';
import { Typewriter } from '@/components/Typewriter';

export default function Home() {
  const { phase, lines, error, mode, setMode, target, setTarget, analyser, toggle } = useDictation();
  const busy = phase !== DictationPhase.Idle;
  const lang = lines.findLast((l) => l.final)?.lang;
  const chip = languageLabel(lang, lines.filter((l) => l.final).map((l) => l.text));

  return (
    <main className="flex h-dvh flex-col">
      <header className="flex items-center justify-between px-6 py-4">
        <h1 className="text-lg font-semibold tracking-tight">Dictation</h1>
        <div className="flex rounded-full bg-rule p-0.5 text-xs">
          {Object.values(DictationMode).map((m) => (
            <button
              key={m}
              disabled={busy}
              onClick={() => setMode(m)}
              className={`rounded-full px-3 py-1 capitalize transition ${mode === m ? 'bg-white text-ink shadow-sm' : 'text-muted'}`}
            >
              {m}
            </button>
          ))}
        </div>
      </header>

      <div className="flex min-h-9 items-center gap-2 px-6 text-xs">
        {chip && <span className="rounded-full bg-accent/10 px-2.5 py-1 font-medium text-accent">{chip}</span>}
        {mode === DictationMode.Translate && (
          <select
            value={target}
            disabled={busy}
            onChange={(e) => setTarget(e.target.value as Lang)}
            className="rounded-full border border-rule bg-white px-2.5 py-1"
          >
            {Object.entries(LANGUAGE_NAMES).map(([code, name]) => (
              <option key={code} value={code}>→ {name}</option>
            ))}
          </select>
        )}
      </div>

      <section className="mx-auto w-full max-w-4xl flex-1 space-y-3 overflow-y-auto px-6 pb-44 pt-2 text-lg leading-relaxed">
        {lines.length === 0 && <p className="text-gutter">Tap the mic and start speaking…</p>}
        {lines.map((l) => (
          <div key={l.id} className="grid gap-x-8 gap-y-1 sm:grid-cols-2">
            <p className={l.final ? '' : 'text-gutter'}>
              <Typewriter text={l.text} instant={!l.final} />
            </p>
            {l.translation && l.translation !== l.text && (
              <p className="text-accent">
                <Typewriter text={l.translation} />
              </p>
            )}
          </div>
        ))}
      </section>

      <div className="pointer-events-none fixed inset-x-0 bottom-3 flex flex-col items-center gap-2 px-4">
        {error && (
          <p role="alert" className="pointer-events-auto rounded-xl border border-margin/30 bg-white shadow-sm px-3 py-2 text-sm text-margin">
            {error}
          </p>
        )}
        <div className="pointer-events-auto">
          <SealRing phase={phase} analyser={analyser} onClick={toggle} />
        </div>
      </div>
    </main>
  );
}
