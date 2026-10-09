# Translate, Speech Output & Legal-Paper UI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Transcribe|Translate and Text|Speech toggles with automatic language detection, behind a provider facade that is free-tier-first (Google, then Sarvam), and replace the UI with the "pleading paper" legal theme.

**Architecture:**
- **Backend:** Go adapters implement three small provider contracts (`SpeechToText`, `Translator`, `TextToSpeech`). A `service.Speech` facade routes each request through ordered provider chains, gated by a persisted `usage.Meter` (Google free quota is a hard stop; Sarvam has a ₹ budget) and a file cache.
- **WebSocket:** carries a JSON config first, then audio, and receives typed JSON events.
- **Frontend:** a Next.js page driven by one reducer state machine, rendering a ruled "pleading page" and a canvas "seal ring" waveform.

**Tech Stack:** Go 1.24 module (`github.com/agnivade/stt_challenge`); gorilla/websocket; testify + mockery; `cloud.google.com/go/speech/apiv1`, `cloud.google.com/go/translate` (v2 basic), `cloud.google.com/go/texttospeech/apiv1`; Sarvam REST and WebSocket; Next.js 15 / React 19 / Tailwind v4; Vitest (new, dev only).

**Spec:** `docs/superpowers/specs/2026-10-09-translate-speech-output-design.md`

## Global Constraints

- Providers: **only `google` and `sarvam`**. Default chain for every capability: `google,sarvam`.
- Google usage is a **hard stop at the free tier**:
  - STT: 60 min/month
  - Translate: 500,000 chars/month
  - TTS: 4,000,000 chars/month
- Sarvam spend cap: `-budget-inr` (default `100`), counted across all time.
- Sarvam prices: STT ₹30/hour, translate ₹0.005/char, TTS ₹3/1,000 chars.
- A Google STT session starts only if at least 300 s of free minutes remain. STT is charged as streamed seconds, rounded up to a multiple of 15 s per session.
- Session limit: 5 minutes.
- The UI never shows provider names, quotas, prices or confidence numbers.
- Error copy is exactly the spec §4.3 strings, reproduced in Task 7.
- Languages: `en, hi, ml, kn, te, or`. Inside the server they are BCP-47 (`hi-IN`); on the wire from the UI they are short codes (`hi`).
- Sarvam models: STT `saaras:v4` over `wss://api.sarvam.ai/speech-to-text/ws`; translate `mayura:v1`; TTS `bulbul:v3` (never v4-flash). Auth header: `api-subscription-key`.
- No automated test may call a real API.
- Theme tokens:

| Token | Value |
|---|---|
| paper | `#fbf8f1` |
| rule | `#e9e2d4` |
| margin | `#b23a3a` |
| ink | `#1f2a44` |
| muted | `#6d655a` |
| gutter | `#b9ae9b` |

  Fonts: EB Garamond (serif) and Inter (sans).

**Two spec clarifications, applied in the spec in the same commit as this plan:**
- Google STT uses `apiv1`, which already has `AlternativeLanguageCodes` and `StreamingRecognitionResult.LanguageCode`. No `v1p1beta1` is needed.
- The `audio` event also carries the `text` of its chunk, so the Speaking caption can follow playback.

## Review Focus

1. **Client disconnects mid-session** (tab closed, Wi-Fi drop): the provider stream must be closed and the streamed seconds charged and persisted. Covered by `TestWebSocket_ClientDisconnectClosesSessionAndCharges` (Task 4).
2. **Bad first frame** (binary, non-JSON or unknown mode): the server must reply `error{bad_config}` and close, never start a paid stream. Covered by `TestParseConfig` and `TestWebSocket_BadConfigClosesWithoutStarting` (Task 4).
3. **Two tabs at once:** concurrent `Allow`/`Charge` must not race or overshoot. Covered by `TestMeter_ConcurrentChargesRaceFree` with `-race` (Task 2).
4. **Corrupt `.usage.json` at startup:** the server must refuse to start rather than silently reset to zero and overspend. Covered by `TestMeter_CorruptFileIsAnError` (Task 2).
5. **Empty or whitespace-only final transcripts** (Google emits these): they must never reach translate or TTS, and must never render blank lines. Covered by `TestSession_SkipsEmptyFinals` (Task 3), `TestSpeak_EmptyTextReturnsNoAudio` (Task 16), and the reducer test `ignores empty final utterances` (Task 6).

---

## File Structure

```
server/
  providers/stt.go, translate.go, tts.go       contracts (Task 1)
  providers/provider.go                        DELETED (Task 4)
  providers/mocks/mocks.go                     regenerated
  audio/resample.go, audio/pcm.go (+tests)     moved from providers/google (Task 1)
  usage/table.go, usage/meter.go (+test)       Task 2
  service/errors.go, service/speech.go, service/session.go (+tests)   Task 3; extended in 12, 16, 19
  service/chunk.go (+test)                     Task 16
  cache/cache.go (+test)                       Task 9
  providers/google/stt.go (+test)              replaces google.go (Task 4)
  providers/google/translate.go (+test)        Task 11
  providers/google/tts.go (+test)              Task 15
  providers/sarvam/client.go, translate.go, tts.go, stt.go (+tests)   Tasks 10, 14, 18
  protocol.go (+test)                          WS message types, parseConfig, errorCode (Task 4)
  websocket.go, server.go, server_test.go      Task 4; extended in 12, 16
  cmd/server/main.go                           Task 4; extended in 12, 16, 19
webapp/
  vitest.config.ts                             Task 5
  lib/protocol.ts (+test)                      Task 5
  lib/dictationClient.ts (+test)               Task 5
  lib/languages.ts (+test)                     Task 7
  hooks/dictationReducer.ts (+test)            Task 6
  hooks/useDictation.ts                        Task 6; extended in 8, 17
  lib/speechPlayer.ts                          Task 17
  components/SegmentedToggle.tsx, TargetLanguagePicker.tsx, LanguageChip.tsx,
             Typewriter.tsx, PleadingPage.tsx, StatusSlip.tsx            Task 7
  components/SealRing.tsx                      Task 8
  app/layout.tsx, app/page.tsx, app/globals.css                          Tasks 7–8
  DELETED: components/{Header,LanguageSelector,RecorderToggleButton,LiveWaveform,TipTapEditor,RecorderContext}.tsx,
           utils/pttLiveSession.ts, public/adalat-logo.svg (Task 8)
```

---

# Phase 1 — Foundation + new UI (Transcribe, Text)

### Task 1: Provider contracts and shared audio package

**Files:**
- Create: `server/providers/stt.go`, `server/providers/translate.go`, `server/providers/tts.go`
- Create: `server/audio/resample.go` (moved verbatim from `server/providers/google/resample.go`, with the function exported), `server/audio/pcm.go`, `server/audio/audio_test.go`
- Modify: `server/providers/google/google.go` (call `audio.Resample`); delete `server/providers/google/resample.go`
- Modify: `server/.mockery.yml` (currently untracked; commit it in this task)

**Interfaces:**
- Produces:
  - Every type in spec §3.2, verbatim: `Task`, `TaskTranscribe`, `TaskTranslateToEnglish`, `STTFeatures`, `STTConfig`, `Transcript`, `SpeechToText`, `TranscriptStream`, `Translator`, `Audio`, `TextToSpeech`.
  - `audio.Resample(pcm []byte, fromHz, toHz int) ([]byte, error)`
  - `audio.Seconds(pcmBytes, sampleRate int) float64` (16-bit mono: `bytes/2/rate`)
  - `audio.TargetRate = 16000`
  - mocks `mocks.MockSpeechToText`, `mocks.MockTranscriptStream`, `mocks.MockTranslator`, `mocks.MockTextToSpeech`

- [ ] **Step 1: Write failing tests** in `server/audio/audio_test.go`:

```go
func TestSeconds(t *testing.T) {
    assert.Equal(t, 1.0, audio.Seconds(32000, 16000))
    assert.Equal(t, 0.5, audio.Seconds(48000, 48000))
    assert.Equal(t, 0.0, audio.Seconds(0, 16000))
}
func TestResample_48kTo16kThirdsLength(t *testing.T) {
    in := make([]byte, 48000*2) // 1 s of silence at 48 kHz
    out, err := audio.Resample(in, 48000, 16000)
    require.NoError(t, err)
    assert.InDelta(t, 16000*2, len(out), 64)
}
func TestResample_OddLengthErrors(t *testing.T) {
    _, err := audio.Resample([]byte{1, 2, 3}, 48000, 16000)
    assert.Error(t, err)
}
```

- [ ] **Step 2: Run** `cd server && go test ./audio/...`. Expected: FAIL (package `audio` does not exist).
- [ ] **Step 3: Implement.** Move the resampler into `audio` with an exported name, add `Seconds` and `TargetRate`, and switch `google.go` to `audio.Resample`. Add the three contract files exactly as in spec §3.2, with doc comments. Leave `providers/provider.go` untouched; Task 4 deletes it.
- [ ] **Step 4: Update `.mockery.yml`** so that under `github.com/agnivade/stt_challenge/providers` the interfaces are `Provider`, `Session`, `SpeechToText`, `TranscriptStream`, `Translator` and `TextToSpeech`. Then run `make mocks`.
- [ ] **Step 5: Run** `cd server && go build ./... && make test`. Expected: all PASS, including the existing google and server tests.
- [ ] **Step 6: Commit:** `git add server && git commit -m "feat(server): provider contracts and shared audio package"`

---

### Task 2: Usage meter

**Files:**
- Create: `server/usage/table.go`, `server/usage/meter.go`, `server/usage/meter_test.go`
- Modify: `server/.gitignore` (add `.usage.json` and `.cache/`)

**Interfaces:**
- Produces:

```go
type Op string
const ( OpSTT Op = "stt"; OpTranslate Op = "translate"; OpTTS Op = "tts" )

type Key struct{ Provider string; Op Op }
type Limit struct {
    FreePerMonth float64 // units: seconds for STT, runes for translate/TTS; 0 = no free tier
    INRPerUnit   float64 // 0 = paid use not allowed
}
var DefaultTable = map[Key]Limit{
    {"google", OpSTT}: {FreePerMonth: 3600},
    {"google", OpTranslate}: {FreePerMonth: 500_000},
    {"google", OpTTS}: {FreePerMonth: 4_000_000},
    {"sarvam", OpSTT}: {INRPerUnit: 30.0 / 3600},
    {"sarvam", OpTranslate}: {INRPerUnit: 0.005},
    {"sarvam", OpTTS}: {INRPerUnit: 0.003},
}
var ErrQuotaExhausted = errors.New("quota exhausted")

func NewMeter(path string, budgetINR float64, table map[Key]Limit, now func() time.Time) (*Meter, error)
func (m *Meter) Allow(provider string, op Op, qty float64) error  // ErrQuotaExhausted, or an error for an unknown key
func (m *Meter) Charge(provider string, op Op, qty float64) error // records usage and persists atomically
func (m *Meter) Report() Report

type Report struct {
    Month     string             `json:"month"` // "2006-01"
    FreeUsed  map[string]float64 `json:"free_used"` // key "google/stt"
    FreeLimit map[string]float64 `json:"free_limit"`
    SpentINR  float64            `json:"spent_inr"`
    BudgetINR float64            `json:"budget_inr"`
}
```

- **Rules:**
  - A key with a free tier is allowed only while `used+qty <= FreePerMonth`; it never falls back to paid.
  - A paid key is allowed only while `spent+qty*INRPerUnit <= budget`.
  - Free counters are keyed by month and reset when `now()` enters a new month. `SpentINR` never resets.
  - A missing file starts at zero. An unreadable or corrupt file is an error from `NewMeter`.
  - All methods are safe for concurrent use.

- [ ] **Step 1: Write failing tests** in `meter_test.go`. Use `t.TempDir()` and a settable clock.
  - `TestMeter_GoogleFreeTierHardStop`: `Allow("google",OpSTT,3600)` → nil; after `Charge(...,3590)`, `Allow(...,11)` → `ErrQuotaExhausted`.
  - `TestMeter_SarvamBudget`: budget 100; `Charge("sarvam",OpTTS,30000)` (₹90); `Allow("sarvam",OpTTS,3334)` → `ErrQuotaExhausted`; `Allow(...,3333)` → nil.
  - `TestMeter_UnknownKeyIsError`: `Allow("groq",OpSTT,1)` returns a non-nil error that is not `ErrQuotaExhausted`.
  - `TestMeter_MonthRolloverResetsGoogleOnly`: charge google 3600 and sarvam 10 tts in October; move the clock to 1 November; google `Allow(...,3600)` → nil; `Report().SpentINR` → `0.03`.
  - `TestMeter_PersistsAcrossRestart`: charge, then `NewMeter` with the same path; the `Report` values are equal.
  - `TestMeter_CorruptFileIsAnError`: write `"{not json"` to the path; `NewMeter` returns an error.
  - `TestMeter_ConcurrentChargesRaceFree`: 50 goroutines × `Charge("sarvam",OpTTS,10)`; `SpentINR` → `1.5` (within 1e-9).
- [ ] **Step 2: Run** `cd server && go test -race ./usage/...`. Expected: FAIL (undefined).
- [ ] **Step 3: Implement** `table.go` and `meter.go`. Use one `sync.Mutex`; persist by writing `path+".tmp"` and then `os.Rename`.
- [ ] **Step 4: Run** `go test -race ./usage/...`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): usage meter with free-tier hard stops and INR budget"`

---

### Task 3: Speech facade — transcription sessions

**Files:**
- Create: `server/service/errors.go`, `server/service/speech.go`, `server/service/session.go`, `server/service/speech_test.go`

**Interfaces:**
- Consumes: Task 1 contracts and mocks; Task 2 `*usage.Meter`, `usage.OpSTT`.
- Produces:
  - Every type in spec §3.4, verbatim: `Mode`, `ModeTranscribe`, `ModeTranslate`, `SessionOptions`, `Utterance`, `Session`, `Speech`.
  - Plus:

```go
var (
    ErrUnsupportedLanguage = errors.New("unsupported language")
    ErrProviderUnavailable = errors.New("provider unavailable")
    ErrConnectionLost      = errors.New("connection lost")
    ErrSessionLimit        = errors.New("session limit reached")
)
type Config struct {
    STT          []providers.SpeechToText
    Translators  []providers.Translator   // used from Task 12
    TTS          []providers.TextToSpeech // used from Task 16
    Meter        *usage.Meter
    SessionLimit time.Duration            // 0 → 5*time.Minute
}
func New(cfg Config) *Service // *Service implements Speech

// Speak returns chunks with their text so the UI caption can follow playback (spec §3.7 audio.text).
type SpokenChunk struct{ Text string; Audio providers.Audio }
// Speech.Speak signature (deviation from spec §3.4, applied to the spec):
//   Speak(ctx context.Context, text, lang string) ([]SpokenChunk, error)
```

- **Behaviour in this task:**
  - **Start:**
    - For each STT in order, skip it if `Meter.Allow(name, OpSTT, SessionLimit.Seconds())` fails. Otherwise open it with `STTConfig{SampleRate, Task: TaskTranscribe}`; if `Open` errors, try the next one.
    - If none succeeds: return `usage.ErrQuotaExhausted` when every skip was a quota skip, otherwise `ErrProviderUnavailable`.
    - `ModeTranslate` with no translators returns `ErrUnsupportedLanguage` (until Task 12).
  - **Session.Write:** checks `Allow(name, OpSTT, secs)` and then `Charge` for each chunk (`secs = audio.Seconds(len, SampleRate)`). On `ErrQuotaExhausted` it closes the stream, and `Next` then returns `usage.ErrQuotaExhausted`.
  - **Session.Close:** closes the stream once, and charges the rounding remainder up to the next multiple of 15 s.
  - **Session.Next:**
    - Maps a `Transcript` to an `Utterance`. IDs are `"u1"`, `"u2"`, …; an ID increments after each final.
    - Skips finals whose `strings.TrimSpace(Text)==""`.
    - Wraps non-EOF stream errors as `ErrConnectionLost`.
    - Returns `ErrSessionLimit` once `SessionLimit` has elapsed since `Start`; the stream is closed by a timer.
  - **Usage():** returns `Meter.Report()`.

- [ ] **Step 1: Write failing tests** (mocks plus a real Meter in a temp dir):
  - `TestStart_UsesFirstProviderWithQuota`: google's meter is exhausted, so sarvam's `Open` is called and google's is not.
  - `TestStart_AllExhaustedReturnsQuotaError`: `errors.Is(err, usage.ErrQuotaExhausted)`.
  - `TestStart_OpenErrorFallsBack`: google `Open` errors, so sarvam is used.
  - `TestStart_TranslateWithoutTranslatorsUnsupported`.
  - `TestSession_InterimThenFinalShareID`: stream yields interim "kal ki", final "kal ki hearing", final "aur"; Utterance IDs are `u1,u1,u2` and `Final` is `false,true,true`.
  - `TestSession_SkipsEmptyFinals`: final `"  "` then final "ok"; one utterance with `ID "u1"`, `Text "ok"`.
  - `TestSession_ChargesRoundedTo15s`: Write 1 s of 16 kHz PCM, then Close; `Report().FreeUsed["google/stt"]` → `15`.
  - `TestSession_StreamErrorIsConnectionLost`.
  - `TestSession_LimitEndsSession`: `SessionLimit: 50*time.Millisecond`, stream `Next` blocks until Close; `Next` returns `ErrSessionLimit`.
  - `TestSession_PaidBudgetStopsMidStream`: sarvam only, budget ₹0.01; Write 2 s → `Next` → `usage.ErrQuotaExhausted`.
- [ ] **Step 2: Run** `go test -race ./service/...`. Expected: FAIL.
- [ ] **Step 3: Implement** `errors.go`, `speech.go` (`New`, `Start`, `Usage`; `Speak` returns `ErrProviderUnavailable` until Task 16) and `session.go`. Guard Close with `sync.Once`.
- [ ] **Step 4: Run** `go test -race ./service/...`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): Speech facade with free-tier-first STT routing"`

---

### Task 4: Google STT adapter and server switchover to the new protocol

**Files:**
- Create: `server/providers/google/stt.go`, `server/providers/google/stt_test.go` (migrate the cases from `google_test.go`, then delete it); `server/protocol.go`, `server/protocol_test.go`
- Delete: `server/providers/google/google.go`, `server/providers/provider.go`; remove `Provider` and `Session` from `.mockery.yml` and regenerate mocks
- Modify: `server/websocket.go`, `server/server.go`, `server/server_test.go`, `server/cmd/server/main.go`, `server/README.md` (flags and environment table from spec §3.8)

**Interfaces:**
- Consumes: Task 3 `service.Speech`, `service.SessionOptions`, the errors; `usage.ErrQuotaExhausted`.
- Produces:
  - `google.NewSTT(client *speech.Client, languages []string) (*STT, error)`. It errors when `len(languages)` is 0 or greater than 4. `languages[0]` is the primary and the rest are `AlternativeLanguageCodes`.
  - `STT` implements `providers.SpeechToText` with `Name()=="google"` and `Supports()=={AutoDetect:true, MaxCandidates:4, Interim:true, Languages: languages}`.
  - Internal seam: the existing `streamingRecognizeClient` interface (kept for mocks).
  - `normalizeLang(s string) string`: `"hi-in"`→`"hi-IN"`, `"en"`→`"en"`.
  - `server.New(port string, speech service.Speech) *Server`; `GET /usage` returns `speech.Usage()` as JSON.
  - `protocol.go`:

```go
type clientConfig struct{ Mode service.Mode; Target string /* BCP-47 */; Speech bool }
func parseConfig(raw []byte) (clientConfig, error)
// accepts {"type":"config","mode":"transcribe"|"translate","target":"en|hi|ml|kn|te|or","output":"text"|"speech"}
// target is required iff mode=="translate"; maps "hi"→"hi-IN" (all six → "<code>-IN")
func errorCode(err error) string // quota_exhausted | unsupported_language | session_limit | connection_lost | provider_unavailable
type utteranceEvent struct{ Type string `json:"type"`; ID string `json:"id"`; Text string `json:"text"`; Lang string `json:"lang"`; Final bool `json:"final"`; Translation string `json:"translation,omitempty"` }
type audioEvent struct{ Type string `json:"type"`; Seq int `json:"seq"`; Last bool `json:"last"`; Mime string `json:"mime"`; Data []byte `json:"data"`; Text string `json:"text"` }
type errorEvent struct{ Type string `json:"type"`; Code string `json:"code"` }
type doneEvent struct{ Type string `json:"type"` }
```

- **WebConn flow:**
  1. Upgrade.
  2. The first frame must be text and pass `parseConfig`. Otherwise write `error{bad_config}` and close without calling `Start`.
  3. `Start(ctx, SessionOptions{SampleRate from the query (default 16000), Mode, Target})`. On error, write `error{errorCode(err)}` and close.
  4. Reader:
     - `START` starts the writer.
     - Binary frames go to `session.Write`.
     - `STOP` calls `session.Close()`.
     - A read error (client gone) calls `session.Close()` and returns.
  5. Writer: loops `session.Next()` and writes `utterance` events. On `io.EOF` it writes `done` and closes. On another error it writes `error{errorCode}` and closes. Task 16 extends this for `Speech` before `done`.
- **main.go:**
  - Flags `-port`, `-stt-chain` (default `google,sarvam`), `-stt-languages` (default `en-IN,hi-IN,ml-IN,kn-IN`), `-budget-inr` (default `100`).
  - Builds the meter at `.usage.json`.
  - Builds only the providers whose credentials exist. If `SARVAM_API_KEY` is unset, log `sarvam disabled: SARVAM_API_KEY not set`. Until Task 18 there is no Sarvam STT yet, so the chain simply skips it.

- [ ] **Step 1: Write failing tests:**
  - `stt_test.go`:
    - `TestOpen_SendsPrimaryAndAlternatives`: the mock stream's first `Send` has `LanguageCode=="en-IN"`, `AlternativeLanguageCodes==[hi-IN ml-IN kn-IN]`, `SampleRateHertz==16000`, `InterimResults==true`.
    - `TestNext_ForwardsInterimAndFinalWithLanguage`: a result with `LanguageCode:"hi-in"` gives `Transcript{Language:"hi-IN"}`.
    - `TestNewSTT_RejectsFiveLanguages`.
    - Plus the migrated EOF, Canceled and resample cases.
  - `protocol_test.go`:
    - `TestParseConfig` (table): valid transcribe/text; translate without a target → error; `target:"xx"` → error; `mode:"dance"` → error; non-JSON → error; `"hi"` → `"hi-IN"`.
    - `TestErrorCode` (table over the five errors plus a wrapped one).
  - `server_test.go`: replace the mock `Provider` with a hand-written `fakeSpeech` implementing `service.Speech` and run against `httptest.NewServer(srv.srv.Handler)`.
    - `TestWebSocket_TranscribeHappyPath`: send config, `START`, one binary frame, `STOP`; receive `utterance{id:"u1",final:true}` then `done`.
    - `TestWebSocket_BadConfigClosesWithoutStarting`: the first frame is binary; receive `error{bad_config}`; `fakeSpeech.started==0`.
    - `TestWebSocket_StartQuotaError`: receive `error{quota_exhausted}`.
    - `TestWebSocket_ClientDisconnectClosesSessionAndCharges`: after config and START the client closes abruptly; within 1 s `fakeSession.closed==true`.
    - `TestUsageEndpoint`: `GET /usage` → 200 with a JSON `budget_inr`.
    - Keep the start/stop test.
- [ ] **Step 2: Run** `make test`. Expected: FAIL (undefined symbols).
- [ ] **Step 3: Implement** `stt.go`, `protocol.go`, the new `websocket.go` and `server.go`, and `main.go`. Delete the old files and regenerate mocks.
- [ ] **Step 4: Run** `make test && make lint`. Expected: PASS, with no vet findings.
- [ ] **Step 5: Manual smoke test** (about 10 s of free Google quota): `make run-server`, then `curl localhost:8081/usage`. Expected: `{"month":"2026-10",...,"budget_inr":100}`.
- [ ] **Step 6: Commit:** `git commit -m "feat(server): typed WS protocol via Speech facade; Google STT with language detection"`

---

### Task 5: Web protocol types and dictation client

**Files:**
- Create: `webapp/vitest.config.ts`, `webapp/lib/protocol.ts`, `webapp/lib/protocol.test.ts`, `webapp/lib/dictationClient.ts`, `webapp/lib/dictationClient.test.ts`
- Modify: `webapp/package.json` (run `pnpm add -D vitest@^3`; add the script `"test": "vitest run"`)

**Interfaces:**
- Produces:

```ts
export type Mode = 'transcribe' | 'translate';
export type Output = 'text' | 'speech';
export type Lang = 'en' | 'hi' | 'ml' | 'kn' | 'te' | 'or';
export const LANGS: Lang[] = ['en', 'hi', 'ml', 'kn', 'te', 'or'];
export type ErrorCode = 'quota_exhausted' | 'provider_unavailable' | 'unsupported_language' | 'connection_lost' | 'bad_config' | 'session_limit' | 'mic_denied';
export type Settings = { mode: Mode; output: Output; target: Lang };
export type ConfigMessage = { type: 'config'; mode: Mode; output: Output; target?: Lang };
export type UtteranceEvent = { type: 'utterance'; id: string; text: string; lang: string; final: boolean; translation?: string };
export type AudioEvent = { type: 'audio'; seq: number; last: boolean; mime: string; data: string; text: string };
export type ServerEvent = UtteranceEvent | AudioEvent | { type: 'error'; code: ErrorCode } | { type: 'done' };
export function buildConfig(s: Settings): ConfigMessage;       // target only when mode==='translate'
export function parseEvent(raw: string): ServerEvent | null;    // null on bad JSON / unknown type / missing fields

// dictationClient.ts
export type RecorderLike = Pick<Recorder, 'addEventListener' | 'removeEventListener' | 'startMediaRecorder' | 'stopMediaRecorder' | 'getAudioSampleRate'>;
export type DictationHandle = { stop(): void; destroy(): void };
export function openDictation(
  opts: { endpoint: string; settings: Settings; recorder: RecorderLike; WebSocketImpl?: typeof WebSocket },
  on: { event(e: ServerEvent): void; closed(clean: boolean): void },
): Promise<DictationHandle>;
```

- **Client behaviour:**
  - Opens `endpoint?sample_rate=<rate>`.
  - On open, sends `JSON.stringify(buildConfig(settings))`, then `START`, then starts the recorder (500 ms chunks).
  - Audio chunks are sent with the existing blob-trim logic from `pttLiveSession.ts`.
  - `stop()` stops the recorder and sends `STOP`, but keeps the socket open for the trailing `utterance`/`audio`/`done` events.
  - `closed(clean)` gets `true` only when `done` was received before close.

- [ ] **Step 1: Write failing tests:**
  - `protocol.test.ts`:
    - `buildConfig omits target in transcribe mode`.
    - `parseEvent parses utterance`.
    - `parseEvent returns null for {"type":"nope"}`.
    - `parseEvent returns null for invalid JSON`.
    - `parseEvent returns null for utterance missing id`.
  - `dictationClient.test.ts`, with a `FakeWebSocket` class that records `sent[]` and exposes `open()`, `message(s)` and `close()`:
    - `sends config then START on open`: `sent[0]` parses to `{type:'config',mode:'translate',output:'text',target:'hi'}` and `sent[1]==='START'`.
    - `stop sends STOP and keeps socket open`.
    - `closed(true) only after done`.
- [ ] **Step 2: Run** `cd webapp && pnpm test`. Expected: FAIL (modules missing).
- [ ] **Step 3: Implement** `vitest.config.ts` (node environment, alias `@` → project root), `protocol.ts` and `dictationClient.ts`.
- [ ] **Step 4: Run** `pnpm test`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(webapp): typed protocol and dictation client"`

---

### Task 6: Dictation state machine

**Files:**
- Create: `webapp/hooks/dictationReducer.ts`, `webapp/hooks/dictationReducer.test.ts`, `webapp/hooks/useDictation.ts`

**Interfaces:**
- Consumes: Task 5 types and `openDictation`; `Recorder` from `webapp/recorder/recorder.ts`.
- Produces:

```ts
export type Phase = 'idle' | 'connecting' | 'listening' | 'stopping' | 'preparing' | 'speaking' | 'error';
export type Line = { id: string; text: string; lang: string; final: boolean; translation?: string };
export type DictationState = {
  phase: Phase; settings: Settings; lines: Line[]; lang?: string;
  error?: ErrorCode; audio: AudioEvent[]; captionSeq: number;
};
export type Action =
  | { type: 'settings'; patch: Partial<Settings> } | { type: 'start' } | { type: 'connected' }
  | { type: 'server'; event: ServerEvent } | { type: 'stop' } | { type: 'closed'; clean: boolean }
  | { type: 'playback'; status: 'started' | 'ended'; seq?: number } | { type: 'fail'; code: ErrorCode };
export const initialState: (settings: Settings) => DictationState;
export function reducer(s: DictationState, a: Action): DictationState;

export function useDictation(): {
  state: DictationState; analyser: AnalyserNode | null;
  toggle(): void;          // idle/error → start; listening → stop; speaking → stop playback
  replay(): void;          // Task 17
  setSettings(p: Partial<Settings>): void;
};
```

- **Reducer rules:**
  - `settings` applies only in `idle` and `error`.
  - `start`: → `connecting`; clears `error` and `audio`; keeps `lines`.
  - `connected`: → `listening`.
  - An `utterance` replaces the line with the same `id`, or appends a new one. `lang` is set from the event. Finals with empty trimmed text are dropped.
  - `stop`: → `preparing` if `settings.output==='speech'`, else `stopping`.
  - An `audio` event is appended to `audio`.
  - `done` while `stopping` → `idle`. While `preparing`, it stays `preparing`; the player moves it on.
  - A server `error` or `fail` → `error` with its code.
  - `closed(false)` while `connecting`, `listening` or `stopping` → `error`, `connection_lost`.
  - `playback started` → `speaking`, with `captionSeq=seq`. `playback ended` → `idle`.
- **Hook:**
  - Owns one `Recorder` and the `DictationHandle`.
  - Persists `settings` to `localStorage['dictation.settings']` inside try/catch.
  - Maps a `getUserMedia` `NotAllowedError` to `fail{mic_denied}`.
  - Exposes the mic `AnalyserNode` (fftSize 256) while `listening`.

- [ ] **Step 1: Write failing reducer tests**:
  - `start from idle goes connecting and clears error`
  - `settings ignored while listening`
  - `interim then final with same id replaces line`
  - `ignores empty final utterances`
  - `stop in text output goes stopping then done goes idle`
  - `stop in speech output goes preparing; done keeps preparing; playback started→speaking; ended→idle`
  - `unclean close while listening is connection_lost`
  - `server error event sets error phase and code`
  - `lines persist across recordings`
  - `translation on final utterance is stored on the line`
- [ ] **Step 2: Run** `pnpm test`. Expected: FAIL.
- [ ] **Step 3: Implement** `dictationReducer.ts`, then `useDictation.ts` (wiring only; no tests for the hook).
- [ ] **Step 4: Run** `pnpm test && pnpm build`. Expected: PASS, and the type-check is clean.
- [ ] **Step 5: Commit:** `git commit -m "feat(webapp): dictation state machine"`

---

### Task 7: Legal-paper theme and page components

**Files:**
- Create: `webapp/lib/languages.ts`, `webapp/lib/languages.test.ts`, `webapp/components/SegmentedToggle.tsx`, `TargetLanguagePicker.tsx`, `LanguageChip.tsx`, `Typewriter.tsx`, `PleadingPage.tsx`, `StatusSlip.tsx`
- Modify: `webapp/app/globals.css` (Tailwind `@theme` tokens from Global Constraints; ruled-paper background utility), `webapp/app/layout.tsx` (`next/font/google` `EB_Garamond` + `Inter` as CSS variables `--font-serif`/`--font-sans`; title "Dictation"; drop Header and RecorderProvider)

**Interfaces:**
- Consumes: Task 5 `Lang`, `ErrorCode`; Task 6 `Line`.
- Produces:

```ts
export const LANGUAGE_NAMES: Record<Lang, string> = { en: 'English', hi: 'हिन्दी', ml: 'മലയാളം', kn: 'ಕನ್ನಡ', te: 'తెలుగు', or: 'ଓଡ଼ିଆ' };
export function languageLabel(lang: string | undefined, finalTexts: string[]): string | null;
// null when lang undefined; "Hinglish" when lang starts with "hi" and ≥30% of words start with [A-Za-z]; else LANGUAGE_NAMES[base] ?? base
export const ERROR_COPY: Record<ErrorCode, string>;
```

- `ERROR_COPY` values, exactly:
  - `quota_exhausted`: "Today's free limit is reached. Try again later."
  - `provider_unavailable` and `connection_lost`: "Couldn't reach the speech service. Please try again."
  - `unsupported_language`: "This language isn't supported for that option yet."
  - `session_limit`: "Recording stopped after 5 minutes."
  - `mic_denied`: "Microphone access is needed to dictate."
  - `bad_config`: "Couldn't reach the speech service. Please try again."
- Component props:
  - `SegmentedToggle<T extends string>({ options: [T, string][]; value: T; onChange(v: T): void; disabled?: boolean; label: string })` renders `role="radiogroup"`.
  - `TargetLanguagePicker({ value, onChange, disabled })`
  - `LanguageChip({ label: string | null })`
  - `Typewriter({ text: string; cps?: number /* 60 */ })` reveals only newly appended characters and renders instantly under `prefers-reduced-motion`.
  - `PleadingPage({ lines: Line[]; placeholder: string; hideText?: boolean })`:
    - Line-numbered gutter and red double margin.
    - Finals in serif; translations in italic `muted` beneath them; interim in `gutter` colour.
    - When a final's translation equals its text, renders a single line (the spec §3.4 one-call path).
    - Auto-scrolls to the end.
    - Copy button (`aria-label="Copy text"`) copies finals and translations joined with `\n`.
  - `StatusSlip({ code?: ErrorCode })`

- [ ] **Step 1: Write failing tests** in `languages.test.ts`:
  - `Hinglish for Latin-heavy hi`: `languageLabel('hi-IN',['Kal ki hearing ke liye documents ready rakhna'])` → `'Hinglish'`.
  - `native name for Devanagari hi`: `'हिन्दी'`.
  - `ml` → `'മലയാളം'`.
  - `undefined` → `null`.
  - `ERROR_COPY has an entry for every ErrorCode`.
- [ ] **Step 2: Run** `pnpm test`. Expected: FAIL.
- [ ] **Step 3: Implement** `languages.ts`, the theme, the fonts and the six components.
- [ ] **Step 4: Run** `pnpm test && pnpm build`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(webapp): pleading-paper theme and page components"`

---

### Task 8: Seal ring and page assembly (Phase 1 demo)

**Files:**
- Create: `webapp/components/SealRing.tsx`
- Modify: `webapp/app/page.tsx`, `webapp/package.json` (remove `@tiptap/react`, `@tiptap/starter-kit`, `@ricky0123/vad-web`), `webapp/README.md` (new env and run instructions, `pnpm test`)
- Delete: the files listed under DELETED in File Structure

**Interfaces:**
- Consumes: Task 6 `useDictation`, `Phase`; Task 7 components.
- Produces: `SealRing({ phase: Phase; analyser: AnalyserNode | null; onClick(): void })`.
- **SealRing:**
  - A `<button>` wrapping a canvas, 128 CSS px, sized for devicePixelRatio. 64 radial bars, ink colour, inner disc.
  - Per phase:

| Phase | Bars | Glyph | `aria-label` |
|---|---|---|---|
| idle | Breathing (amplitude 2–4 px, ~1.2 rad/s) | Mic | "Start dictation" |
| connecting | Breathing, 2× speed | Mic | "Start dictation" |
| listening | From `getByteFrequencyData` (amplitude ≤ 18 px) | Stop square | "Stop dictation" |
| stopping / preparing | Orbiting highlight | ⚖ | — |
| speaking | Analyser-driven, margin red | Pause bars | "Stop playback" |
| error | Idle drawing at opacity 0.35 | — | — |

  - Under `prefers-reduced-motion`, draws a static ring.
- **page.tsx layout:**
  - Top bar: "⚖ Dictation" in small caps, Mode toggle, Output toggle.
  - Second row: `LanguageChip`, plus `→ TargetLanguagePicker` only in translate mode.
  - `PleadingPage` fills the height.
  - `StatusSlip` sits above the ring; `SealRing` is docked bottom-centre.
  - Toggles are disabled unless the phase is `idle` or `error`.
  - **In this phase the Translate and Speech options are rendered disabled.** Tasks 13 and 17 enable them.
  - Placeholder: "Tap the seal and begin speaking…".

- [ ] **Step 1: Delete the old components and dependencies, then run** `pnpm install && pnpm build`. Expected: the build fails only where `page.tsx` imports deleted files.
- [ ] **Step 2: Implement** `SealRing.tsx` and the new `page.tsx`.
- [ ] **Step 3: Run** `pnpm test && pnpm build && pnpm lint`. Expected: PASS.
- [ ] **Step 4: Manual checklist.** With `make run-server` and `pnpm dev`:
  - (a) The idle ring breathes, and no provider or quota text appears anywhere.
  - (b) Speaking Hindi shows a grey interim line, then the final types in. The chip reads "हिन्दी" or "Hinglish".
  - (c) Stop returns to idle.
  - (d) Denying the mic shows "Microphone access is needed to dictate."
  - (e) Killing the server mid-recording shows "Couldn't reach the speech service. Please try again."
  - (f) At 375 px width there is no horizontal scroll.
  - (g) With reduced motion on, text appears instantly and the ring is static.
- [ ] **Step 5: Commit:** `git commit -m "feat(webapp): seal ring and new dictation screen; remove legacy UI"`

---

# Phase 2 — Translate

### Task 9: Result cache

**Files:**
- Create: `server/cache/cache.go`, `server/cache/cache_test.go`

**Interfaces:**
- Produces:
  - `cache.New(dir string) (*Cache, error)`
  - `(*Cache).Get(key string) ([]byte, bool)`
  - `(*Cache).Put(key string, val []byte) error`
  - `cache.Key(parts ...string) string`: hex sha256 of the parts joined by `"\x00"`.
- Files are stored at `dir/<key[:2]>/<key>` and written atomically (tmp then rename).

- [ ] **Step 1: Write failing tests:**
  - `TestCache_MissThenHit`
  - `TestCache_SurvivesNewInstance`
  - `TestKey_StableAndPartSensitive`: `Key("a","bc") != Key("ab","c")`, and the same inputs give the same output.
- [ ] **Step 2: Run** `go test ./cache/...`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./cache/...`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): file cache for translate/TTS results"`

---

### Task 10: Sarvam client and translate adapter

**Files:**
- Create: `server/providers/sarvam/client.go`, `server/providers/sarvam/translate.go`, `server/providers/sarvam/translate_test.go`

**Interfaces:**
- Produces:

```go
type Options struct {
    APIKey  string
    BaseURL string // default "https://api.sarvam.ai"
    WSURL   string // default "wss://api.sarvam.ai" (Task 18)
    HTTP    *http.Client // default: 15 s timeout
}
func NewTranslator(o Options) *Translator // Name()=="sarvam"
```

- `client.go` holds `postJSON(ctx, path string, body, out any) error`. It sets the header `api-subscription-key`, and maps non-2xx responses to `fmt.Errorf("sarvam %s: %d %s", path, status, body)`.
- `Translate` sends `POST /translate` with `{"input":text,"source_language_code":src,"target_language_code":target,"model":"mayura:v1","mode":"formal"}` and returns `translated_text`.
- `Supports(src, target)`: both are in `{en,hi,ml,kn,te,or}-IN` and they differ.

- [ ] **Step 1: Write failing tests** with `httptest.NewServer`:
  - `TestTranslate_RequestShapeAndAuth`: asserts the path, the header and the JSON body fields; replies `{"translated_text":"Keep the documents ready","source_language_code":"hi-IN"}`; the result equals that text.
  - `TestTranslate_Non2xxIsError` (500).
  - `TestSupports` (table: `hi-IN→en-IN` true, `or-IN→kn-IN` true, `hi-IN→hi-IN` false, `ta-IN→en-IN` false).
- [ ] **Step 2: Run** `go test ./providers/sarvam/...`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): Sarvam Mayura translate adapter"`

---

### Task 11: Google translate adapter

**Files:**
- Create: `server/providers/google/translate.go`, `server/providers/google/translate_test.go`
- Modify: `server/go.mod` (`go get cloud.google.com/go/translate@latest`), `server/.mockery.yml` (add `translateClient` under the google package), regenerate mocks

**Interfaces:**
- Produces:
  - `google.NewTranslator(c translateClient) *Translator`, where `translateClient` is `interface{ Translate(ctx context.Context, inputs []string, target language.Tag, opts *translate.Options) ([]translate.Translation, error) }`. `*translate.Client` satisfies it.
  - `Name()=="google"`.
  - `Supports(src,target)`: same six-language rule as Sarvam.
  - `Translate` strips regions (`"hi-IN"`→`hi`), passes `Options{Source: hi, Format: translate.Text}` and returns `[0].Text`.

- [ ] **Step 1: Write failing tests:**
  - `TestTranslate_StripsRegionAndUsesTextFormat`: the mock asserts `target==language.English` and `opts.Source==language.Hindi`, and checks the format.
  - `TestTranslate_ClientErrorPropagates`.
  - `TestTranslate_EmptyResultIsError`.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): Google Translate adapter"`

---

### Task 12: Facade translation and wiring

**Files:**
- Modify: `server/service/speech.go`, `server/service/session.go`, `server/service/speech_test.go`, `server/cmd/server/main.go` (flag `-translate-chain`; build `cache.New(".cache")`; Google translate client from `translate.NewClient`; Sarvam translator when the key is set), `server/README.md`

**Interfaces:**
- Consumes: Task 9 cache; Tasks 10–11 translators.
- Produces:
  - `Config.Cache *cache.Cache`, a new field; nil disables caching.
  - `(*Service).translate(ctx, text, src, target string) (string, error)` (unexported).
  - In `ModeTranslate`, `Session.Next` fills `Utterance.Translation` on finals only.
- **Rules:**
  - When `base(src)==base(target)`, `Translation=Text` and no call is made.
  - For each translator in the chain with `Supports(src,target)`:
    1. Look up the cache key `Key(name,"translate",src,target,text)`. A hit is returned.
    2. Otherwise check `Meter.Allow(name, OpTranslate, runes)`, call `Translate`, `Put` the result in the cache, and `Charge`.
    3. On an error or quota failure, continue to the next translator.
  - No supporting translator → `ErrUnsupportedLanguage`. All failed → the last error, mapped to `ErrProviderUnavailable` unless every failure was a quota failure.
  - `Start` in `ModeTranslate` returns `ErrUnsupportedLanguage` only when no translator supports any source for that target. The per-utterance error is returned from `Next` and ends the session.

- [ ] **Step 1: Write failing tests:**
  - `TestTranslate_SameLanguageSkipsCall` (the mock translator expects no calls).
  - `TestTranslate_GoogleQuotaFallsBackToSarvam`.
  - `TestTranslate_CacheHitSkipsMeterAndCall`: the second identical utterance makes 1 call total, and `FreeUsed` is unchanged after the second.
  - `TestTranslate_InterimNotTranslated`.
  - `TestTranslate_UnsupportedPairErrors`.
- [ ] **Step 2: Run** `go test -race ./service/...`. Expected: FAIL.
- [ ] **Step 3: Implement**, and wire `main.go`.
- [ ] **Step 4: Run** `make test`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): translate mode with cache and free-tier fallback"`

---

### Task 13: Enable Translate in the UI

**Files:**
- Modify: `webapp/app/page.tsx` (enable the Translate option and `TargetLanguagePicker`)

**Interfaces:**
- Consumes: Tasks 5–8. The reducer already stores `translation` (tested in Task 6).
- Produces: no new API.

- [ ] **Step 1: Implement** the enablement. The default target is `en`, and the picker shows native names from `LANGUAGE_NAMES`.
- [ ] **Step 2: Run** `pnpm test && pnpm build`. Expected: PASS.
- [ ] **Step 3: Manual check:** speak Hinglish with target English; italic English appears under each line. Set the target to Hindi and speak Hindi; there is no duplicate italic line, because the server skips translation for the same language.
- [ ] **Step 4: Commit:** `git commit -m "feat(webapp): enable translate mode"`

---

# Phase 3 — Speech output

### Task 14: Sarvam TTS adapter

**Files:**
- Create: `server/providers/sarvam/tts.go`, `server/providers/sarvam/tts_test.go`

**Interfaces:**
- Produces:
  - `sarvam.NewTTS(o Options) *TTS`, implementing `providers.TextToSpeech` with `Name()=="sarvam"` and `MaxChars()==2500`.
  - `Supports(lang)` is true for `{en,hi,ml,kn,te,or}-IN`.
  - `Synthesize` sends `POST /text-to-speech` with `{"text":t,"language_code":lang,"model":"bulbul:v3","output_audio_codec":"mp3"}`, base64-decodes `audios[0]` and returns `Audio{MimeType:"audio/mpeg"}`.

- [ ] **Step 1: Write failing tests:**
  - `TestTTS_RequestShape` (model `bulbul:v3`, codec mp3, header).
  - `TestTTS_DecodesFirstAudio`.
  - `TestTTS_EmptyAudiosIsError`.
  - `TestTTS_SupportsOdia`.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): Sarvam Bulbul v3 TTS adapter"`

---

### Task 15: Google TTS adapter

**Files:**
- Create: `server/providers/google/tts.go`, `server/providers/google/tts_test.go`
- Modify: `server/go.mod` (`go get cloud.google.com/go/texttospeech@latest`), `.mockery.yml` (add `ttsClient`), regenerate mocks

**Interfaces:**
- Produces:
  - `google.NewTTS(c ttsClient) *TTS`, where `ttsClient` is `interface{ SynthesizeSpeech(ctx context.Context, req *texttospeechpb.SynthesizeSpeechRequest, opts ...gax.CallOption) (*texttospeechpb.SynthesizeSpeechResponse, error) }`.
  - `Name()=="google"`; `MaxChars()==1500` (the request limit is 5,000 bytes, and Indic scripts use 3 bytes per character).
  - Voice map: `en-IN→en-IN-Wavenet-A`, `hi-IN→hi-IN-Wavenet-A`, `kn-IN→kn-IN-Wavenet-A`, `ml-IN→ml-IN-Wavenet-A`, `te-IN→te-IN-Standard-A`.
  - `Supports(lang)` is true only for keys in the voice map, so Odia is false.
  - Encoding MP3 → `Audio{MimeType:"audio/mpeg"}`.

- [ ] **Step 1: Write failing tests:**
  - `TestTTS_UsesVoiceMapAndMP3` (asserts `Voice.Name`, `LanguageCode`, `AudioEncoding_MP3`).
  - `TestTTS_DoesNotSupportOdia`.
  - `TestTTS_ClientErrorPropagates`.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): Google TTS adapter"`

---

### Task 16: Facade Speak and speech output over WebSocket

**Files:**
- Create: `server/service/chunk.go`, `server/service/chunk_test.go`
- Modify: `server/service/speech.go`, `server/service/speech_test.go`, `server/websocket.go`, `server/server_test.go`, `server/cmd/server/main.go` (flag `-tts-chain`; Google TTS client from `texttospeech.NewClient`; Sarvam TTS), `server/README.md`

**Interfaces:**
- Consumes: Tasks 9, 14, 15.
- Produces:
  - `splitForTTS(text string, max int) []string`: splits after `.`, `!`, `?` or `।` followed by space or end of text, then packs whole sentences up to `max` runes. A sentence longer than `max` is split at the last space before `max`, or hard-split if there is none. It returns no empty chunks.
  - `Speak(ctx, text, lang) ([]providers.Audio, error)`:
    - Returns `nil, nil` for whitespace-only text.
    - Picks the first TTS in the chain that `Supports(lang)` and passes `Allow` for the **whole text's** rune count, so all chunks share one provider and one voice. The cache is checked per chunk with `Key(name,"tts",lang,chunk)`.
    - On a mid-way provider error, restarts from the next provider.
    - Returns `[]SpokenChunk` (defined in Task 3), one per chunk, in order.
- **WebConn:**
  - When `cfg.Speech`, the writer collects final texts: `Translation` in translate mode, otherwise `Text`. It records the language as the target in translate mode, otherwise the language of the most frequent final.
  - After `io.EOF` it calls `Speak(strings.Join(texts," "), lang)` and writes one `audio{seq,last,mime,data,text}` per chunk, then `done`.

- [ ] **Step 1: Write failing tests:**
  - `chunk_test.go`:
    - `TestSplitForTTS_PacksSentences` (`"A. B. C."`, max 5 → `["A. B.","C."]`)
    - `TestSplitForTTS_DandaBoundary`
    - `TestSplitForTTS_LongSentenceSplitsAtSpace`
    - `TestSplitForTTS_NoEmptyChunks`
  - `speech_test.go`:
    - `TestSpeak_EmptyTextReturnsNoAudio`
    - `TestSpeak_OdiaSkipsGoogle`
    - `TestSpeak_GoogleQuotaFallsBackToSarvam`
    - `TestSpeak_CachedChunksSkipCall`
  - `server_test.go`: `TestWebSocket_SpeechOutputSendsAudioThenDone`, using a `fakeSpeech` that returns 2 chunks; the frames are `utterance`, `audio{seq:0,last:false}`, `audio{seq:1,last:true}`, `done`.
- [ ] **Step 2: Run** `make test`. Expected: FAIL.
- [ ] **Step 3: Implement**, and wire `main.go`.
- [ ] **Step 4: Run** `make test && make lint`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): speech output via TTS chain"`

---

### Task 17: Speech player, speaking states and Replay in the UI

**Files:**
- Create: `webapp/lib/speechPlayer.ts`
- Modify: `webapp/hooks/useDictation.ts` (drive `playback` actions; expose the player analyser while speaking; `replay()`), `webapp/hooks/dictationReducer.ts` (add `{type:'replay'}` → `speaking` with `captionSeq:0` when `audio.length>0`), `webapp/hooks/dictationReducer.test.ts`, `webapp/app/page.tsx` (enable the Speech option; in `speech` output pass `hideText` to `PleadingPage` with the placeholder "Listening… this will be read back to you."; in `preparing`, two shimmer lines; in `speaking`, one italic caption with `audio[captionSeq].text` and a single "Replay" text button, visible again after playback ends)

**Interfaces:**
- Consumes: Task 6 reducer; the `AudioEvent` type.
- Produces:

```ts
export class SpeechPlayer {
  constructor(ctx?: AudioContext);
  readonly analyser: AnalyserNode;
  load(chunks: AudioEvent[]): Promise<void>;           // decodeAudioData per chunk, in seq order
  play(onChunk: (seq: number) => void): Promise<void>; // resolves when the last chunk ends
  stop(): void;
}
```

- **Hook:** when the phase is `preparing` and an `audio` event with `last:true` has arrived, it calls `load`, then `play`, dispatching `playback started` per chunk (with its seq) and `playback ended` when done. `toggle()` while `speaking` calls `stop()` and dispatches `playback ended`.

- [ ] **Step 1: Write failing reducer tests:** `replay with audio goes speaking at seq 0` and `replay without audio is ignored`.
- [ ] **Step 2: Run** `pnpm test`. Expected: FAIL.
- [ ] **Step 3: Implement** `speechPlayer.ts`, the hook wiring and the page states.
- [ ] **Step 4: Run** `pnpm test && pnpm build`, then the manual check:
  - Speech + Transcribe in Hindi: after Stop the page clears and shimmers, then the ring turns red and moves with the audio, and the caption follows each chunk.
  - Replay works.
  - Clicking the ring while speaking stops playback.
  - Speech + Translate with target Odia plays (via Sarvam).
- [ ] **Step 5: Commit:** `git commit -m "feat(webapp): speech output playback with seal ring"`

---

# Phase 4 — Sarvam STT fallback

### Task 18: Sarvam streaming STT adapter

**Files:**
- Create: `server/providers/sarvam/stt.go`, `server/providers/sarvam/stt_test.go`

**Interfaces:**
- Consumes: Task 10 `Options`; Task 1 `audio.Resample`, `audio.TargetRate`.
- Produces:
  - `sarvam.NewSTT(o Options) *STT`, with `Name()=="sarvam"` and `Supports()=={AutoDetect:true, MaxCandidates:0, CodeMix:true, TranslateToEnglish:true, Interim:false, Languages: the six}`.
  - `Open` dials `WSURL+"/speech-to-text/ws?language-code=unknown&model=saaras:v4&mode=<codemix|translate>&sample_rate=16000&input_audio_codec=pcm_s16le"` with the header `Api-Subscription-Key`. The mode is `translate` iff `cfg.Task==TaskTranslateToEnglish`.
  - `Write` resamples to 16 kHz and sends `{"audio":{"data":<base64>,"sample_rate":"16000","encoding":"audio/wav"}}`.
  - `Next`:
    - `{"type":"data","data":{transcript,language_code,language_probability}}` → `Transcript{Final:true, Language, LanguageConfidence: parsed float (0 if null)}`.
    - `type:"events"` messages are skipped.
    - `type:"error"` → `fmt.Errorf("sarvam stt: %s", data.error)`.
    - A normal close, or a close after `Close()`, → `io.EOF`.
  - `Close` sends `{"type":"flush"}` and closes the connection after `flushGrace` (a package var, default `2*time.Second`; tests set it to 50 ms) **or** after the next `data` message, whichever is first. It is idempotent.

- [ ] **Step 1: Write failing tests** against an `httptest` server using `websocket.Upgrader`:
  - `TestOpen_QueryAndHeader` (codemix for Transcribe, translate for TaskTranslateToEnglish, `language-code=unknown`).
  - `TestWrite_SendsBase64AudioMessage`.
  - `TestNext_ParsesDataWithLanguage` (`language_probability:"0.92"` → `0.92`).
  - `TestNext_SkipsEvents`.
  - `TestNext_ErrorMessage`.
  - `TestClose_FlushesThenEOF`.
- [ ] **Step 2: Run** `go test -race ./providers/sarvam/...`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "feat(server): Sarvam Saaras v4 streaming STT adapter"`

---

### Task 19: One-call speech→English and STT chain wiring

**Files:**
- Modify: `server/service/speech.go`, `server/service/session.go`, `server/service/speech_test.go`, `server/cmd/server/main.go` (add Sarvam STT to the `-stt-chain` registry), `server/README.md`, `README.md` (root: env vars `SARVAM_API_KEY`; enable Google Translation and TTS APIs; recommend a ₹1 GCP budget alert; `-stt-chain=sarvam,google` for better Hinglish)

**Interfaces:**
- Consumes: Task 18 adapter; Task 12 translate path.
- Produces, as changes to `Start` and `Session.Next`:
  - In `ModeTranslate` with `base(Target)=="en"`, a selected STT whose `Supports().TranslateToEnglish` is true is opened with `TaskTranslateToEnglish`. Its finals then get `Translation=Text` and no Translator call is made.
  - Otherwise the behaviour is unchanged.

- [ ] **Step 1: Write failing tests:**
  - `TestStart_TranslateToEnglishUsesSingleCall`: the sarvam STT mock expects `Open` with `Task==TaskTranslateToEnglish`, the translator mock expects 0 calls, and `Utterance.Translation==Utterance.Text`.
  - `TestStart_TranslateToHindiStillUsesTranslator`.
  - `TestStart_GoogleSTTTranslateToEnglishUsesTranslator` (Google lacks the feature).
- [ ] **Step 2: Run** `go test -race ./service/...`. Expected: FAIL.
- [ ] **Step 3: Implement**, and wire `main.go`.
- [ ] **Step 4: Run** `make test && make lint`, then `cd ../webapp && pnpm test && pnpm build`. Expected: all PASS.
- [ ] **Step 5: Manual check:** start the server with `-stt-chain=sarvam,google` and speak Hinglish in Translate→English; each line appears once, in English. Check that `curl localhost:8081/usage` shows `spent_inr` increasing slightly and staying below `100`.
- [ ] **Step 6: Commit:** `git commit -m "feat(server): one-call speech-to-English via Sarvam; configurable STT chain"`
