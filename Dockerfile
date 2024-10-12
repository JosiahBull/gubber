FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o /gubber

FROM alpine:3.18 AS runner
# update and install dependencies
RUN apk update && \
    apk add --no-cache git rdiff-backup

COPY --from=builder /gubber /gubber

ENTRYPOINT [ "/gubber" ]
