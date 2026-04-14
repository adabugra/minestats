# MineStats: Minecraft Server Tracker

Fast Minecraft server tracking with:
- Go backend for high-frequency status packet polling
- SQLite persistence for time-series history

## Run Backend

```bash
cd backend
go run . -- -config config.json
```

Backend listens on `http://localhost:8080` by default.

## Run Frontend

```bash
cd frontend
pnpm install
pnpm dev
```

Frontend defaults to backend at `http://localhost:8080`.

## Run with Docker

Setup commands:

```bash
cp .env.example .env
cp config.json.example config.json
docker compose up -d --build
```

Default result:
- Frontend on `0.0.0.0:5000`
- Backend on `127.0.0.1:8080` (private to host by default)
- SQLite persisted in Docker volume `minestats_data`

All deploy knobs are in `.env`:
- `APP_BIND_IP`, `APP_PORT`: public frontend binding
- `BACKEND_BIND_IP`, `BACKEND_PORT`: optional direct backend host binding
- `VITE_API_BASE`, `VITE_WS_BASE`: optional build-time overrides (leave empty for same-origin). If set, include `http://` or `https://` (frontend also auto-adds `http://` if omitted).
