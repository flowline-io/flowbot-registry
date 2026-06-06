FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG BUILD_DATE=unknown
ARG VCS_REF=unknown
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X github.com/flowline-io/flowbot-registry/version.Buildstamp=${BUILD_DATE} -X github.com/flowline-io/flowbot-registry/version.Buildtags=${VCS_REF}" \
    -o flowbot-registry ./cmd/server

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /opt/app

COPY --from=builder /src/flowbot-registry .

RUN chmod +x flowbot-registry

ARG VERSION=dev
ARG BUILD_DATE=unknown
ARG VCS_REF=unknown

LABEL org.opencontainers.image.title="flowbot-registry" \
      org.opencontainers.image.description="OCI-based plugin registry for the Flowbot ecosystem" \
      org.opencontainers.image.url="https://github.com/flowline-io/flowbot-registry" \
      org.opencontainers.image.source="https://github.com/flowline-io/flowbot-registry" \
      org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.created=${BUILD_DATE} \
      org.opencontainers.image.revision=${VCS_REF} \
      org.opencontainers.image.licenses="GPL-3.0"

EXPOSE 3000

ENTRYPOINT ["./flowbot-registry"]
