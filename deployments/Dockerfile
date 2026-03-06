FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /flowgate ./cmd/flowgate

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /flowgate /flowgate
COPY config/flowgate.yaml /etc/flowgate/flowgate.yaml

ENV FLOWGATE_CONFIG=/etc/flowgate/flowgate.yaml

EXPOSE 8080

ENTRYPOINT ["/flowgate"]
