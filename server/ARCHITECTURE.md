# Architecture

```
Browser ──WebSocket──► internal/server ──► internal/service ──► internal/providers/{google,sarvam}
  config JSON, PCM,      handler.go          Start(): first STT that opens
  "STOP"                 protocol.go         Next(): utterances; in translate mode each
  ◄── utterance/done/error events            final is translated by the first translator that succeeds
```

1. The client's first text frame is `{"type":"config","mode":"transcribe"|"translate","target":"en|hi|ml|kn|te|mr"}`.
2. Binary frames are 16-bit PCM at `?sample_rate=` (default 16000); the Google adapter resamples to 16 kHz.
3. The server sends `utterance{id,text,lang,final,translation?}` events (interim and final share an `id`), then `done` after `STOP`, or `error{code}`.
4. Providers are tried in order: Google first, then Sarvam. A provider without credentials is skipped.
