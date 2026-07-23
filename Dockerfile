# Single Dockerfile for all Go services. It builds all three binaries; the
# running service is chosen at runtime with `command:` (docker-compose) or
# `dockerCommand` (Render), so no build args are needed (Render Blueprints do
# not support them). renderall runs the ledger and gateway combined in one
# process; it's used only by the free Render deploy, see services/renderall.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ledger ./services/ledger/cmd/ledger && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./services/gateway && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/renderall ./services/renderall

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ledger /ledger
COPY --from=build /out/gateway /gateway
COPY --from=build /out/renderall /renderall
# Default command; override per service (compose `command`, Render `dockerCommand`).
CMD ["/gateway"]
