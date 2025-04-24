FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build -o main ./cmd
RUN go build -o db-cli ./cmd/database

FROM alpine:latest

RUN apk add --no-cache curl postgresql-client

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/db-cli .
COPY --from=builder /app/cmd/database/seeder/data /app/cmd/database/seeder/data
COPY --from=builder /app/internal/utils/mailing/template /app/internal/utils/mailing/template

RUN mkdir -p logs

CMD ["/bin/sh", "-c", "/app/db-cli -migrate -seed && /app/main"]
