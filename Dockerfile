# Stage 1: Compile static Go FinOps Gateway binary
FROM golang:1.24-alpine AS builder

ENV GOTOOLCHAIN=auto \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

COPY go.mod ./
COPY . .

RUN go build -ldflags="-s -w" -o /out/finsort_server ./cmd/server

# Stage 2: Minimal Alpine runtime
FROM alpine:3.20 AS runtime

RUN apk --no-cache add ca-certificates wget tzdata

WORKDIR /app

COPY --from=builder /out/finsort_server /app/finsort_server

ENV PORT=8080 \
    PYTHON_AI_URL=http://utho-ai:8001

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=4s --start-period=5s --retries=5 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

CMD ["/app/finsort_server"]
