# Translate, Speech Output & Legal-Paper UI — Design

- **Date:** 2026-10-09
- **Status:** Approved in brainstorming; awaiting written-spec review
- **Purpose:** Personal learning project. Explore multi-provider speech AI (STT, translation, TTS) for Indian languages while keeping real spend at ₹0 (Google free tier) and at most ₹100 (Sarvam credits).

## 1. Goals

1. Add a **Transcribe | Translate** toggle. In Translate mode the user picks a target language; the source language is auto-detected.
2. Add a **Text | Speech** output toggle. In Speech mode the result is spoken back after the user stops recording, instead of being shown as text.
3. Detect the spoken language automatically (no source dropdown), including code-mixed speech such as Hinglish.
4. Support multiple providers behind clean interfaces. One vendor may do several steps in a single call (e.g. speech → English text).
5. **Cost is the top priority.** Use Google's free tier first, then Sarvam's ₹100 free credits. The server enforces both limits; it is never left to discipline.
6. Replace the current UI with a calm "pleading paper" legal theme: a circular waveform ring as the only control, and text that writes itself onto a ruled page.

### Non-goals

- Live (per-sentence) speech output while still talking. Speech plays only after Stop.
- Editing the transcript. The page is read-only, with a Copy button.
- Showing provider names, quotas, prices or confidence numbers in the UI.
- Providers other than Google and Sarvam. The `fake`, Groq, Gemini and browser-TTS providers are out of scope.
- Swapping providers mid-utterance.
- User accounts or a server-side transcript history.

### Languages

`en`, `hi`, `ml`, `kn`, `te`, `or`, plus Hinglish (code-mixed `hi` + `en`). These are both the target languages and the detected languages.

## 2. Current state (relevant findings)

- `server/websocket.go` hardcodes `LanguageCode: "en-US"`. The `language` query parameter the client sends is ignored.
- The server sends JSON `{sentence, confidence}`, but `webapp/utils/pttLiveSession.ts` dispatches the raw `e.data` string. TipTap therefore inserts raw JSON into the editor.
- `providers/google/google.go` returns only final results; interim results are dropped.
- `providers.Provider` / `providers.Session` are STT-only and generically named. They have no notion of capabilities, language detection or translation.
- `@ricky0123/vad-web` is a dependency but is not imported anywhere.

## 3. Backend design (Go, `server/`)

### 3.1 Package layout

```
server/
  providers/
    stt.go            SpeechToText, TranscriptStream, STTConfig, STTFeatures, Transcript, Task
    translate.go      Translator
    tts.go            TextToSpeech, Audio
    google/           STT (existing, migrated) + Translate + TTS adapters
    sarvam/           STT (WebSocket) + Translate (Mayura) + TTS (Bulbul v3) adapters
    mocks/            mockery-generated
  audio/              resample.go (moved from providers/google), PCM/WAV helpers
  service/            Speech facade: routing, single-call combination, chunking
  usage/              Meter (free quotas + ₹ spend), price/quota table, persistence
  cache/              file cache for translate + TTS results
  server.go, websocket.go, cmd/server/main.go
```

`providers.Provider` and `providers.Session` are removed and replaced by the interfaces below.

### 3.2 Provider interfaces

```go
// providers/stt.go
type Task int
const (
    TaskTranscribe Task = iota
    TaskTranslateToEnglish
)

type STTFeatures struct {
    AutoDetect         bool     // can detect the language without a hint
    MaxCandidates      int      // max languages considered during detection (0 = unlimited)
    CodeMix            bool     // emits natural code-mixed text (Hinglish)
    TranslateToEnglish bool     // supports TaskTranslateToEnglish in the same call
    Interim            bool     // emits non-final transcripts
    Languages          []string // BCP-47 codes supported
}

type STTConfig struct {
    SampleRate int
    Candidates []string // languages to consider; empty = provider decides
    Task       Task
}

type Transcript struct {
    Text               string
    Language           string  // detected BCP-47, e.g. "hi-IN"
    LanguageConfidence float32
    Final              bool
    Provider           string
}

type SpeechToText interface {
    Name() string
    Supports() STTFeatures
    Open(ctx context.Context, cfg STTConfig) (TranscriptStream, error)
}

type TranscriptStream interface {
    Write(pcm []byte) error        // 16-bit PCM at cfg.SampleRate
    Next() (Transcript, error)     // blocks; io.EOF when the stream ends
    Close() error
}

// providers/translate.go
type Translator interface {
    Name() string
    Supports(src, target string) bool
    Translate(ctx context.Context, text, src, target string) (string, error)
}

// providers/tts.go
type Audio struct {
    Data     []byte
    MimeType string // "audio/mpeg" or "audio/wav"
}

type TextToSpeech interface {
    Name() string
    Supports(lang string) bool
    MaxChars() int // per-request limit; the facade chunks to it
    Synthesize(ctx context.Context, text, lang string) (Audio, error)
}
```

### 3.3 Adapters

| Adapter | Capability | Notes |
|---|---|---|
| `google` STT | `SpeechToText` | `apiv1` streaming (already has `AlternativeLanguageCodes`). Primary language plus up to 3 `alternative_language_codes` (`MaxCandidates=4`). Interim results now forwarded (`Interim=true`). No translate task. |
| `google` Translate | `Translator` | Translation v2 Basic. |
| `google` TTS | `TextToSpeech` | WaveNet voices for `en-IN`, `hi-IN`, `ml-IN`, `kn-IN`, `te-IN`. **No Odia.** |
| `sarvam` STT | `SpeechToText` | Saaras v4 over WebSocket (`/speech-to-text/ws`). `language_code=unknown` auto-detects all 22 Indic languages plus English. Mode is `codemix` for Transcribe and `translate` for TranslateToEnglish. Final results only (`Interim=false`). |
| `sarvam` Translate | `Translator` | Mayura; any pair among the 11 supported languages. |
| `sarvam` TTS | `TextToSpeech` | Bulbul **v3** (v4-flash lacks Malayalam and Odia). 2,500 characters per request. |

Each adapter declares its language support in a static table in its package. During implementation, verify the tables against vendor docs.

### 3.4 `service.Speech` facade

`WebConn` depends only on this interface:

```go
type Mode int
const ( ModeTranscribe Mode = iota; ModeTranslate )

type SessionOptions struct {
    SampleRate int
    Mode       Mode
    Target     string // required when Mode == ModeTranslate
}

type Utterance struct {
    ID          string
    Text        string // original-language text
    Language    string // detected
    Final       bool
    Translation string // set on final utterances in Translate mode
}

type Session interface {
    Write(pcm []byte) error
    Next() (Utterance, error) // io.EOF at end
    Close() error
}

type Speech interface {
    Start(ctx context.Context, opts SessionOptions) (Session, error)
    Speak(ctx context.Context, text, lang string) ([]SpokenChunk, error) // chunked; SpokenChunk{Text string; Audio providers.Audio}
    Usage() usage.Report
}
```

**Routing.** Each capability has an ordered chain. The default is `google,sarvam` for all three; it is configurable with the flags `-stt-chain`, `-translate-chain` and `-tts-chain`. For each request the facade selects the first provider that:
1. supports the language (or language pair), and
2. passes `meter.Allow(provider, op, estimatedQty)` (free quota left, or ₹ budget left).

**Single-call combination.** In `ModeTranslate` with `Target == "en"`, if the selected STT provider has `TranslateToEnglish`, the facade opens the stream with `TaskTranslateToEnglish` and skips the Translator. Only Sarvam can do this today. On this path the original-language text is not available, so `Utterance.Text` and `Utterance.Translation` both hold the English text. The page renders a single line for such utterances.

**Skip redundant translation.** If the detected language equals the target, `Translation = Text` and no call is made.

**Speak.** Splits text at sentence boundaries into chunks of at most `MaxChars()`, synthesizes each chunk (cache first) and returns the chunks in order. Language is chosen as follows:
- Translate mode: the target.
- Transcribe mode: the dominant detected language of the session.

**Fallback.**
- Translate and TTS: on a provider error or `ErrQuotaExhausted`, try the next provider in the chain.
- STT: fallback happens only at `Start`. A mid-stream failure ends the session with an error.

**Session limit.** At most 5 minutes per session. This also sits under Google's ~305 s single-stream limit. The facade closes the session at the limit, and the client shows "Recording stopped after 5 minutes."

### 3.5 Usage meter (`usage/`)

- **Quotas and prices.** These live in one Go table; the values come from vendor pricing pages, 2026-10-09.

| Provider/op | Free quota (resets monthly) | Paid price |
|---|---|---|
| google/stt | 60 min | not allowed (hard stop) |
| google/translate | 500,000 chars | not allowed (hard stop) |
| google/tts (WaveNet) | 4,000,000 chars | not allowed (hard stop) |
| sarvam/stt | — | ₹30 / hour |
| sarvam/translate | — | ₹0.005 / char |
| sarvam/tts | — | ₹3 / 1,000 chars |

- **Budget.** Sarvam spend is capped at `-budget-inr` (default `100`), counted across all time.
- **Accounting rules.**
  - Google STT: a session may start only if at least 5 free minutes remain. Usage is charged as streamed seconds, rounded up to 15 s per session (conservative).
  - Sarvam STT: charged per streamed second while streaming. The session is closed with `quota_exhausted` when the budget is reached.
  - Translate and TTS: `Allow` is checked against the known character count before the call, and `Charge` runs after success.
- **API.** `Allow(provider, op, qty) error` (returns `ErrQuotaExhausted`), `Charge(provider, op, qty)` and `Report()`.
- **Persistence.** Saved to `server/.usage.json` (gitignored). The file is written atomically after every charge; Google counters are keyed by month.
- **Endpoint.** `GET /usage` returns the report as JSON. It is for the developer only; the UI does not show it.

### 3.6 Cache (`cache/`)

A file cache in `server/.cache/` (gitignored). The key is `sha256(provider, op, lang/src→target, voice, text)`. It applies to Translate and TTS only. A cache hit makes no provider call and no meter charge.

### 3.7 WebSocket protocol

**Client → server**
1. The first text frame is the config:
   `{"type":"config","mode":"transcribe"|"translate","target":"en"|"hi"|"ml"|"kn"|"te"|"or","output":"text"|"speech"}`. `target` is present only when mode is `translate`. The server replies `error{bad_config}` and closes if the config is invalid.
2. `START`: begin streaming (unchanged).
3. Binary frames: audio (unchanged format; the server resamples via `audio/`).
4. `STOP`: end of speech. In `output=speech` the server synthesizes and then closes.

**Server → client** (JSON text frames, all with `type`)
- `utterance`: `{id, text, lang, final, translation?}`. Interim and final utterances share an `id`; a final one replaces the interim text.
- `audio`: `{seq, last, mime, data, text}`. `data` is base64; `text` is the chunk's text, which drives the Speaking caption. Sent only after `STOP` when `output=speech`.
- `error`: `{code}`, where code is `quota_exhausted | provider_unavailable | unsupported_language | connection_lost | bad_config | session_limit`.
- `done`: the session finished normally.

`sample_rate` stays as a URL query parameter. The `language` query parameter is removed.

### 3.8 Configuration

| Flag / env | Default | Purpose |
|---|---|---|
| `-port` | `8081` | unchanged |
| `-stt-chain`, `-translate-chain`, `-tts-chain` | `google,sarvam` | provider order |
| `-stt-languages` | `en-IN,hi-IN,ml-IN,kn-IN` | Google STT primary + 3 alternatives |
| `-budget-inr` | `100` | Sarvam spend cap |
| `GOOGLE_APPLICATION_CREDENTIALS` | — | Google; requires the STT, Translation and TTS APIs enabled |
| `SARVAM_API_KEY` | — | Sarvam; if unset, Sarvam is removed from every chain with a startup log line |

## 4. Frontend design (Next.js, `webapp/`)

### 4.1 Visual language: "pleading paper" (approved mockup L1)

- Ivory page `#fbf8f1` with faint horizontal rules every line, line numbers in the left gutter, and a red double margin rule `#b23a3a`.
- Ink accent: navy `#1f2a44`. Body text is EB Garamond (serif) for transcript content and Inter for controls, both via `next/font`.
- Wordmark "⚖ Dictation" in small caps. The black header and Adalat logo are removed.
- Works from phone width up, with no horizontal scroll.

### 4.2 Layout

The top bar holds the wordmark, the `Transcribe | Translate` toggle and the `Text | Speech` toggle. Below it are the language chip and, in Translate mode only, the target picker (`→ English ▾`). The ruled page fills the remaining height, with the ring docked bottom-centre over it.

### 4.3 States

| State | Page | Ring | Other |
|---|---|---|---|
| idle | Italic placeholder: "Tap the seal and begin speaking…" | Slow breathing, mic glyph | — |
| connecting | unchanged | Breathing, faster | — |
| listening (Text) | Finals type in; translation in italics beneath each; interim shown in grey | Driven by mic analyser; stop glyph | Chip shows detected language (e.g. "Hinglish", "हिन्दी") |
| listening (Speech) | No transcript is shown (speech replaces text); placeholder reads "Listening… this will be read back to you." | same | Chip still shows detected language |
| preparing | Page clears; two shimmer lines | Slow orbiting highlight | — |
| speaking | One faint italic caption line following playback | Driven by playback analyser, margin red; pause glyph | Single "Replay" control |
| error | Unchanged | Dimmed | Plain-language message slip above the ring |

No provider names, quotas, prices or confidence numbers appear anywhere. The error copy is:
- `quota_exhausted`: "Today's free limit is reached. Try again later."
- `provider_unavailable` / `connection_lost`: "Couldn't reach the speech service. Please try again."
- `unsupported_language`: "This language isn't supported for that option yet."
- `session_limit`: "Recording stopped after 5 minutes."
- Mic denied: "Microphone access is needed to dictate."

### 4.4 Components

| Unit | Responsibility |
|---|---|
| `components/SegmentedToggle.tsx` | Generic two-option pill (used for both toggles); disabled while recording |
| `components/TargetLanguagePicker.tsx` | `en, hi, ml, kn, te, or` with native-script labels |
| `components/LanguageChip.tsx` | Displays the detected language name; hidden until the first utterance. Shows "Hinglish" when `lang` is `hi-*` and at least 30% of the words in the session's final text are in Latin script; otherwise shows the native name (e.g. "हिन्दी") |
| `components/PleadingPage.tsx` | Ruled, numbered sheet; renders the utterance list; auto-scroll; Copy button |
| `components/Typewriter.tsx` | Character-by-character reveal of new text; instant when `prefers-reduced-motion` is set |
| `components/SealRing.tsx` | Canvas circular waveform; props `{analyser?: AnalyserNode, state, onClick}`; keyboard and ARIA accessible |
| `components/SpeechPlayer.ts` | Decodes and queues `audio` chunks in an `AudioContext`; exposes an `AnalyserNode` and `replay()` |
| `components/StatusSlip.tsx` | Maps an error code to the plain message |
| `hooks/useDictation.ts` | Reducer-based state machine (states above); owns the recorder, client and player |
| `lib/dictationClient.ts` | Typed protocol client (replaces `utils/pttLiveSession.ts`); sends config first; parses events |
| `lib/protocol.ts` | Shared TS types for config and events; a pure `parseEvent()` |

- Toggle and target choices persist in `localStorage` inside try/catch. They are locked while recording and take effect on the next recording.
- `recorder/recorder.ts` stays as is.

**Removed:** `Header.tsx`, `LanguageSelector.tsx`, `RecorderToggleButton.tsx`, `LiveWaveform.tsx`, `TipTapEditor.tsx`, `RecorderContext.tsx`, `utils/pttLiveSession.ts`, and the dependencies `@tiptap/*` and `@ricky0123/vad-web`.

## 5. Testing

**Go (`make test`)**
- `service/`: facade tests with mockery mocks:
  - chain order
  - language-unsupported skip
  - quota-exhausted fallback
  - single-call translate-to-English path
  - same-language translation skip
  - TTS chunking and order
  - 5-minute session limit
  - mid-stream STT failure → `connection_lost`
- `usage/`: allow and charge, monthly reset for Google, all-time Sarvam budget, Google STT 5-minute start guard, persistence round-trip.
- `cache/`: hit, miss and key stability.
- `providers/google`: existing tests migrated to the new interfaces, plus interim forwarding and alternative-language mapping.
- `providers/sarvam`: `httptest` fakes for the REST (translate, TTS) and WebSocket (STT) endpoints, covering request shape, response parsing and error mapping.
- `server_test.go`: protocol tests for config validation, utterance events, and speech output after `STOP`.

**Web**
- Add Vitest (dev dependency) for `useDictation` reducer transitions and `parseEvent()`.
- Manual visual checklist for each state in §4.3, including phone width and reduced motion.

**No automated test calls a real API.** A manual smoke run uses a few seconds of free Google quota.

## 6. Delivery phases

Each phase ends with a working demo.

1. **Foundation + new UI (Transcribe, Text):**
   - New provider interfaces, `audio/`, `usage/`, `service/`, and the new protocol.
   - Google STT migrated: hardcoded language removed, interim results forwarded, detection over up to 3 alternatives.
   - The full L1 UI with ring, typewriter, chip and toggles. Translate and Speech are visible but disabled.
   - The raw-JSON bug is fixed by the new protocol.
2. **Translate:** `cache/`, Google Translate and Sarvam Mayura adapters; target picker enabled; translations rendered.
3. **Speech output:** Google TTS and Sarvam Bulbul v3 (Odia always on Sarvam); `SpeechPlayer`; preparing and speaking states; Replay.
4. **Sarvam STT fallback:** Saaras v4 WebSocket adapter with auto-detection and codemix; the single-call speech→English path; `-stt-chain` ordering.

## 7. Known limitations and risks

- **Detection when Google STT is first:** it only considers 4 configured languages. Telugu or Odia speech is mis-transcribed until the chain is flipped (`-stt-chain=sarvam,google`) or Google's free minutes run out.
- **Hinglish on Google:** output may mix scripts. Sarvam `codemix` gives cleaner Hinglish.
- **Sarvam STT has no interim results:** text appears per pause, not mid-sentence. The UI simply has no grey line.
- **Prices drift:** the quota/price table is a single source in `usage/`, to be re-checked against vendor pages before relying on it.
- **Google requires a billing-enabled project** even for free-tier use. The meter's hard stops prevent charges, but the cloud console budget alert should also be set to ₹1 as a backstop.
