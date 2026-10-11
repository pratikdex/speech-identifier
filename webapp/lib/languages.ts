export enum Lang {
  English = 'en',
  Hindi = 'hi',
  Malayalam = 'ml',
  Kannada = 'kn',
  Telugu = 'te',
  Marathi = 'mr',
}

export const ALL_LANGUAGES: readonly Lang[] = Object.values(Lang);

export const LANGUAGE_NAMES: Record<Lang, string> = {
  [Lang.English]: 'English',
  [Lang.Hindi]: 'हिन्दी',
  [Lang.Malayalam]: 'മലയാളം',
  [Lang.Kannada]: 'ಕನ್ನಡ',
  [Lang.Telugu]: 'తెలుగు',
  [Lang.Marathi]: 'मराठी',
};

export function isSupportedLang(code: string): code is Lang {
  return Object.values(Lang).includes(code as Lang);
}

// languageLabel names the detected language; Hindi with mostly-Latin words reads as Hinglish.
export function languageLabel(lang: string | undefined, texts: string[]): string | null {
  if (!lang) return null;
  const base = lang.split('-')[0].toLowerCase();
  if (base === Lang.Hindi) {
    const words = texts.join(' ').split(/\s+/).filter(Boolean);
    const latin = words.filter((w) => /^[A-Za-z]/.test(w)).length;
    if (words.length > 0 && latin / words.length >= 0.3) return 'Hinglish';
  }
  return (isSupportedLang(base) ? LANGUAGE_NAMES[base] : null) ?? base;
}
