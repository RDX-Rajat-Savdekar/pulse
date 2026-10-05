FROM golang:1.26.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG CMD=ingest
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pulse ./cmd/${CMD}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -u 65532 pulse
COPY --from=build /out/pulse /pulse
USER 65532
EXPOSE 8080
ENTRYPOINT ["/pulse"]
