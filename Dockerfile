FROM golang:1.25.1-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/ci-agent \
    ./cmd/ci-agent


FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget

RUN addgroup -S ciagent && adduser -S ciagent -G ciagent

WORKDIR /app

COPY --from=builder /out/ci-agent /app/ci-agent

USER ciagent

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/ci-agent"]
CMD ["serve", "--listen", ":8080"]
