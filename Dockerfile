FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o wrenderer ./cmd/server/

FROM chromedp/headless-shell:latest

WORKDIR /app

RUN apt update \
    && apt install -y ca-certificates \
    && apt clean \
    && apt autoclean

COPY --from=builder /app/wrenderer .

ENTRYPOINT [ "./wrenderer" ]
