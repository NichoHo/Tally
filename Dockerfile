# Single Dockerfile for both Go services. It builds both binaries; the running
# service is chosen at runtime with `command:` (docker-compose) or `dockerCommand`
# (Render), so no build args are needed (Render Blueprints do not support them).
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ledger ./services/ledger/cmd/ledger && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./services/gateway

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ledger /ledger
COPY --from=build /out/gateway /gateway
# Default command; override per service (compose `command`, Render `dockerCommand`).
CMD ["/gateway"]
