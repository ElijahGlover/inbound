FROM golang:alpine as build

# Install git & deps
RUN apk add --no-cache git gcc ca-certificates tzdata

# Copy source
WORKDIR /go/src/github.com/elijahglover/inbound
COPY . .

# Compile supervisor
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o bin/inbound ./cmd/inbound

# Output is scratch image
FROM scratch
ENV SERVICE_HTTP 80
ENV SERVICE_HTTPS 443
ENV LOG_LEVEL verbose
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /go/src/github.com/elijahglover/inbound/bin/inbound /inbound

EXPOSE 80
EXPOSE 443
EXPOSE 8080
EXPOSE 8081

ENTRYPOINT ["/inbound"]