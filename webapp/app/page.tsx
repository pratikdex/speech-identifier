"use client";
import RecorderToggleButton from "@/components/RecorderToggleButton";
import TipTapEditor from "../components/TipTapEditor";
import LanguageSelector from "@/components/LanguageSelector";
import LiveWaveform from "@/components/LiveWaveform";



export default function Home() {
  return (
    <div className="flex flex-col h-full gap-3">
      <div className="flex items-center justify-between gap-3">
          <LanguageSelector />
          <RecorderToggleButton />
      </div>
      <div className="flex-1">
        <TipTapEditor />
      </div>
      <div>
        <LiveWaveform />
      </div>
    </div>
  );
}
