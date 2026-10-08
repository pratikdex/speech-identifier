# Architecture Overview

## System Overview

The system follows a client-server architecture with multi-provider support for speech-to-text transcription. The main components are:

1. **Client** - Captures audio, handles deduplication, and displays transcriptions
2. **WebSocket Server** - Manages connections and coordinates providers
4. **STT Providers** - Interface with external speech services (Google)

## Architecture Diagram

```
┌─────────────────┐    WebSocket     ┌─────────────────────────────────┐
│     Client      │◄────────────────►│         Server                  │
│                 │     JSON         │                                 │
│ - Audio Capture │     Messages     │ ┌─────────────────────────────┐ │
│ - Display Text  │                  │ │        WebConn              │ │
│                 │                  │ │                             │ │
└─────────────────┘                  │ │                             │ │
                                     │ └─────────────┬───────────────┘ │
                                     └───────────────┼─────────────────┘
                                                     │
                                                   │
                                      ┌────────────┼
                                      ▼            ▼
                               ┌──────────────┐ ┌──────────────┐
                               │   Google     │ │   Future     │
                               │  Provider    │ │  Provider    │
                               │              │ │              │
                               └──────────────┘ └──────────────┘
```

## Data Flow

### 1. Audio Input → Provider Distribution
- Client captures audio and sends via WebSocket to Server
- WebConn receives audio data and forwards to Session

### 2. Provider Processing → Result Collection
- Each provider processes audio independently (Google via gRPC)
- Providers send transcription results back to client
