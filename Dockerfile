FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /sift-agent ./cmd/sift-agent

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /sift-agent /usr/local/bin/sift-agent
USER nobody:nobody
ENTRYPOINT ["sift-agent"]
