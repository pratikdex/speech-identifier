# Speech-to-Text Challenge

A real-time speech-to-text transcription system with support for multiple providers (Google Cloud Speech-to-Text). The system consists of a WebSocket server that processes audio streams and a client that captures audio from microphone or file input.

## Features

- **Multi-provider support**: Google Cloud Speech-to-Text
- **Real-time transcription**: WebSocket-based streaming audio processing

## Directory Structure

```
stt-challenge/
├── cmd/
│   └── server/           # Server application
│       └── main.go       # Server entry point
├── providers/            # Speech provider implementations
│   ├── provider.go       # Provider interfaces
│   ├── google/           # Google Speech-to-Text provider
│   └── mocks/            # Generated mocks for testing
├── server.go             # HTTP server and connection management
├── websocket.go          # WebSocket connection handling
├── *_test.go            # Test files
├── Makefile             # Build and run commands
└── README.md            # This file
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

#### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_APPLICATION_CREDENTIALS` | For Google provider | Path to Google Cloud service account JSON file |


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
