# MineStats: Multi Minecraft Server Tracker

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

```bash
docker compose up --build
```

After startup:
- Frontend HTTP: `http://localhost`
- Frontend HTTPS: `https://<your-domain>` (when `SITE_ADDRESS` is a real domain)

Notes:
- `config.json` is mounted into backend as read-only.
- SQLite data is persisted in Docker volume `minestats_data`.
- Backend is exposed on host port `8080`.
- Frontend is exposed on host ports `80` and `443`.
- To enable automatic HTTPS, run compose with a domain:

```bash
SITE_ADDRESS=example.com docker compose up --build
```

If `SITE_ADDRESS` is left unset, frontend serves HTTP on port `80`.
