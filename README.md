# Interview Challenge - Real-Time Speech-to-Text System

A full-stack real-time speech transcription system consisting of a Go WebSocket server and a Next.js web application for live dictation.

## System Overview

This project demonstrates a complete speech-to-text pipeline:
- **Server** (`/server`): Go-based WebSocket server with Google Cloud Speech-to-Text integration
- **Webapp** (`/webapp`): Next.js frontend with real-time audio capture and transcription display

## Quick Start

### Server Setup

1. Navigate to the server directory:
   ```bash
   cd server
   ```

2. Set up Google Cloud credentials:
   ```bash
   export GOOGLE_APPLICATION_CREDENTIALS="path/to/your/service-account.json"
   ```

3. Start the server:
   ```bash
   make run-server
   # or manually: go run ./cmd/server
   ```

The server will run on port 8081 by default.

### Webapp Setup

1. Navigate to the webapp directory:
   ```bash
   cd webapp
   ```

2. Install dependencies:
   ```bash
   npm install -g pnpm
   pnpm install
   ```

3. Create `.env.local` file:
   ```bash
   NEXT_PUBLIC_DICTATION_ENDPOINT=ws://localhost:8081/ws
   ```

4. Start the development server:
   ```bash
   pnpm dev
   ```

The webapp will be available at `http://localhost:3000`.

To use the app, simply click on the "Start recording" button and start speaking. As you give gaps in your speech, you should start seeing transcripts appear in your screen.

## Architecture

- **Backend**: Go server with WebSocket support for real-time audio streaming
- **Frontend**: Next.js with TypeScript and Tailwind CSS for the user interface
- **Communication**: WebSocket connection for bidirectional audio/text streaming
- **Speech Provider**: Google Cloud Speech-to-Text API integration

## Prerequisites

- Go 1.24+
- Node.js 18+
- Google Cloud Platform account with Speech-to-Text API enabled
- Valid Google Cloud service account with appropriate permissions

## Testing

### Server Tests
```bash
cd server
make test
```

### Development Commands
```bash
# Server: Generate mocks
cd server && make mocks

# Webapp: Run development server
cd webapp && pnpm dev
```

For detailed component-specific information, see:
- [Server Documentation](./server/README.md)
- [Webapp Documentation](./webapp/README.md)
