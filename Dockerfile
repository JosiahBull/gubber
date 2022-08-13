# syntax=docker/dockerfile:1
FROM golang:1.18

# update and install dependencies
RUN apt-get update
RUN apt-get install -y git

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o /gubber

ENTRYPOINT [ "/gubber" ]