# Build the izapplebasic telegram bot daemon.
# The build context is the repository root, see docker-compose.yml.
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /izapplebasic-telegram ./cmd/izapplebasic-telegram

# distroless/static has the CA certificates needed for the
# connection to api.telegram.org
FROM gcr.io/distroless/static-debian12
COPY --from=builder /izapplebasic-telegram /izapplebasic-telegram
ENTRYPOINT ["/izapplebasic-telegram", "-data", "/data"]
