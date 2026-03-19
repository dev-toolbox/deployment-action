FROM golang:1.24.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/deployment-action ./cmd/deployment-action

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/deployment-action /usr/local/bin/deployment-action

ENTRYPOINT ["/usr/local/bin/deployment-action"]