# SSD VOIP Setup TUI

Build the configuration binary from the project root:

```bash
go build -o tui/setup-tui ./tui
```

Run it from the project root so it edits the root `.env`:

```bash
./tui/setup-tui
```

The TUI supports editing the ARI, module WebSocket, call, and Piper settings, saving them to `.env`, and testing the ARI HTTP endpoint.
