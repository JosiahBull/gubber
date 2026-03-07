FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o /gubber

FROM alpine:3.23 AS runner
# update and install dependencies
RUN apk update && \
    apk add --no-cache git rdiff-backup

COPY --from=builder /gubber /gubber

ENTRYPOINT [ "/gubber" ]
