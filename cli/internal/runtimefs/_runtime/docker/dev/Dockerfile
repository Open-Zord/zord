
FROM golang:1.26-alpine

ENV GO111MODULE=on

WORKDIR /app
RUN apk add git

COPY . .

RUN apk add curl
ENV ATLAS_VERSION=v0.29.0
RUN curl -sSf https://atlasgo.sh | sh

RUN go mod download
RUN go install github.com/air-verse/air@v1.61.1
RUN apk add build-base

CMD ["air", "http"]
