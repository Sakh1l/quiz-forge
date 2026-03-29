FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o quiz-forge ./cmd/server

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/quiz-forge .
COPY --from=builder /app/static ./static

RUN mkdir -p /var/lib/quiz-forge

EXPOSE 8080

CMD ["./quiz-forge"]
