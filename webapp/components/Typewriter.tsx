'use client';

import { useEffect, useRef, useState } from 'react';

// Typewriter reveals text a character at a time, continuing from whatever
// prefix was already shown (so an interim line turning final doesn't retype).
export function Typewriter({ text, instant = false }: { text: string; instant?: boolean }) {
  const [shown, setShown] = useState(instant ? text.length : 0);
  const prev = useRef(text);

  useEffect(() => {
    let common = 0;
    while (common < Math.min(prev.current.length, text.length) && prev.current[common] === text[common]) common++;
    prev.current = text;
    setShown((s) => (instant ? text.length : Math.min(s, common)));
  }, [text, instant]);

  useEffect(() => {
    if (shown >= text.length) return;
    const id = setTimeout(() => setShown((s) => s + 1), 16);
    return () => clearTimeout(id);
  }, [shown, text]);

  return <>{text.slice(0, shown)}</>;
}
