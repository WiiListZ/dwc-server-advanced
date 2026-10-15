# Server image. The web panel is prebuilt in api/dist, so no Node.js is needed.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X dwc-server-advanced/common.Version=${VERSION}" -o /out/dwc-server-advanced .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/dwc-server-advanced /usr/local/bin/dwc-server-advanced
COPY config_example.yml /usr/local/share/dwc-server-advanced/config_example.yml

# /data holds config.yml, certificates, payload/, state/ and logs/.
VOLUME /data
ENV DWC_HOME=/data \
    DWC_SETUP_ADDRESS=0.0.0.0:80 \
    DWC_UPDATE_MODE=notify
WORKDIR /data

# NAS and website, HTTPS, GameSpy services, QR2 and NAT negotiation.
EXPOSE 80 443 28910 29900 29901 29920 27900/udp 27901/udp

ENTRYPOINT ["dwc-server-advanced"]
