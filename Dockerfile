FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o wrenderer ./cmd/server/
RUN go build -o wrenderer-worker ./cmd/worker/

FROM chromedp/headless-shell:stable

WORKDIR /app

RUN apt update \
    && apt install -y ca-certificates \
    && apt clean \
    && apt autoclean

COPY --from=builder /app/wrenderer .
COPY --from=builder /app/wrenderer-worker .

ENTRYPOINT [ "./wrenderer" ]
