# Speech-to-Text Challenge

A WebSocket server for real-time dictation: streams microphone audio to speech-to-text (Google) and optionally translates each sentence (Google, falling back to Sarvam).

## Features

- **Providers**: Google Speech-to-Text and Translate; Sarvam Translate as fallback
- **Language detection**: Google STT with up to 3 alternative languages
- **Real-time transcription**: WebSocket-based streaming audio processing

## Directory Structure

```
server/
├── cmd/server/main.go        # Entry point: flags, provider wiring, HTTP server
└── internal/
    ├── server/               # WebSocket handler and protocol messages
    ├── service/              # Speech facade: STT session + translation, providers tried in order
    ├── providers/            # Provider interfaces (stt.go, translate.go)
    │   ├── google/           # Google STT + Translate
    │   ├── sarvam/           # Sarvam Translate
    │   └── mocks/            # Generated mocks (make mocks)
    └── audio/                # PCM helpers and resampling
```

## Prerequisites

- Go 1.24+
- API credentials for at least one provider:
  - **Google Cloud**: `GOOGLE_APPLICATION_CREDENTIALS` environment variable

## Quick Start

### Using Makefile

```bash
# Run server (both providers enabled by default)
make run-server

# Run tests
make test

# Generate mocks
make mocks
```

### Manual Commands

```bash
# Start server
go run ./cmd/server

```

## Usage

### Server

Start the server with different provider configurations:

```bash
go run ./cmd/server

# Run on custom port
go run ./cmd/server -port=8080
```

#### Server Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-port` | string | `"8081"` | Server port |
| `-origin` | string | `"http://localhost:3000"` | Browser origin allowed to open sessions |
| `-stt-languages` | string | `"en-IN,hi-IN,ml-IN,kn-IN"` | Google STT primary language plus up to 3 alternatives |


#### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_APPLICATION_CREDENTIALS` | For Google provider | Path to Google Cloud service account JSON file; requires the Speech-to-Text, Translation and Text-to-Speech APIs enabled |
| `SARVAM_API_KEY` | For Sarvam provider | Sarvam API key; translation fallback after Google; skipped if unset |

Put these in `server/.env` (copy `.env.example`); the server loads it at startup. Providers are tried in order (Google, then Sarvam); one whose credentials are missing is skipped.


## Development

### Running Tests

```bash
make test
```

### Generating Mocks

```bash
make mocks
```

## Architecture

For a detailed explanation of the system architecture and data flow, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Contributing

1. Run tests before submitting: `make test`
2. Generate mocks if interfaces change: `make mocks`
3. Follow Go best practices and existing code style
4. Add tests for new functionality

## License

This project is part of a coding challenge and is for demonstration purposes.
