# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/fitlog ./cmd/fitlog

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 fitlog
USER fitlog
WORKDIR /app
COPY --from=build /out/fitlog /usr/local/bin/fitlog

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/fitlog"]
CMD ["server"]
