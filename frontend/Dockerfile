FROM node:24.22.1-alpine@sha256:06d7adf3e0ef61a13f1d28682726366f97a15aa1888c934d3ecec86c33918b53 AS builder
WORKDIR /app

COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile

COPY . .

ARG VITE_API_BASE=
ARG VITE_WS_BASE=
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_WS_BASE=$VITE_WS_BASE

RUN pnpm build

FROM caddy:2.11.7-alpine@sha256:eb4d35b7452d1ab6171fbdbe8d9275b2301dad6dad92115d91e19fb9b24d510f

COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=builder /app/dist /usr/share/caddy

EXPOSE 80
