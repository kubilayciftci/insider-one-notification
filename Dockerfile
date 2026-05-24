FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /worker ./cmd/worker

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /api /usr/local/bin/api
COPY --from=builder /worker /usr/local/bin/worker
COPY --from=builder /app/migrations /migrations

EXPOSE 8080

CMD ["api"]
