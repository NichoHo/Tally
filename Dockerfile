# Single Dockerfile for both Go services. SERVICE_PATH selects which main to
# build (services/ledger/cmd/ledger or services/gateway).
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE_PATH
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./${SERVICE_PATH}

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
