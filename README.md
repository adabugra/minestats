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
