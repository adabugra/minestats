FROM golang:1.26-bookworm@sha256:47ce5636e9936b2c5cbf708925578ef386b4f8872aec74a67bd13a627d242b19 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/minestats .

FROM gcr.io/distroless/static-debian12@sha256:20bc6c0bc4d625a22a8fde3e55f6515709b32055ef8fb9cfbddaa06d1760f838
WORKDIR /app

COPY --from=builder /out/minestats /app/minestats

EXPOSE 8080

ENTRYPOINT ["/app/minestats"]
CMD ["-config", "/app/config.json"]
