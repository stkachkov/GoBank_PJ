FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
ARG SERVER_PORT=8080
EXPOSE ${SERVER_PORT}
CMD ["sleep", "infinity"]
ENTRYPOINT ["./main"]
