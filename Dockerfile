# syntax=docker/dockerfile:1
FROM golang:1.18

# update and install dependencies
RUN apt-get update
RUN apt-get install -y git rdiff-backup

WORKDIR /app

COPY . .

# remove any files that match the gitignore
RUN git clean -Xdf

RUN go mod download

RUN go build -o /gubber

ENTRYPOINT [ "/gubber" ]