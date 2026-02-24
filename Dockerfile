FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /pdf-studio ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /pdf-studio /app/pdf-studio
COPY migrations /app/migrations
COPY web /app/web

RUN mkdir -p /data/uploads /data/pdfs

EXPOSE 8080

CMD ["/app/pdf-studio"]

#claude --resume 7281bc36-2bc1-4175-8e83-e501c62365e4 
