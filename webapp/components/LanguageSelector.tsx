'use client';

import { useRecorder } from "./RecorderContext";

export default function LanguageSelector() {
  const { language, setLanguage } = useRecorder();
  
  return (
    <label className="inline-flex items-center gap-2 text-sm">
      <span className="font-medium">Language:</span>
      <select
        className="border rounded-md px-2 py-1"
        value={language}
        onChange={(e) => setLanguage(e.target.value)}
      >
        <option value="en">English</option>
        <option value="ml">മലയാളം</option>
        <option value="hi">हिन्दी</option>
        <option value="kn">ಕನ್ನಡ</option>
        <option value="te">తెలుగు</option>
        <option value="or">ଓଡ଼ିଆ</option>
      </select>
    </label>
  );
}
