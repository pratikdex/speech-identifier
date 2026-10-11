# Dictation demo

A single-screen dictation app: tap the seal, speak, and the transcript types
onto a ruled "pleading paper" page. It talks to the Go server in `../server`
over a WebSocket.

## Configuration

Create `.env.local` with:

- `NEXT_PUBLIC_DICTATION_ENDPOINT`: the server's WebSocket URL, e.g.
  `ws://localhost:8081/ws` (the server's default port is 8081).

## Running

Start the server first (see `../server/README.md` for credentials):

```bash
cd ../server && make run-server
```

Then the webapp:

```bash
npm install -g pnpm
pnpm install
pnpm dev
```

Open http://localhost:3000 and allow microphone access.

## Testing

```bash
pnpm build   # production build and type check
```

## Tech stack

- Next.js, React, TypeScript
- Tailwind CSS
