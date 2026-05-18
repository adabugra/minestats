FROM golang:1.26-bookworm@sha256:252599aeb51ad60b83e4d8821802068127c528c707cb7dd7afd93be057c6011c AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/minestats .

FROM gcr.io/distroless/static-debian12@sha256:9c346e4be81b5ca7ff31a0d89eaeade58b0f95cfd3baed1f36083ddb47ca3160
WORKDIR /app

COPY --from=builder /out/minestats /app/minestats

EXPOSE 8080

ENTRYPOINT ["/app/minestats"]
CMD ["-config", "/app/config.json"]
