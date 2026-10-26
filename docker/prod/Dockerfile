FROM golang:1.26-alpine AS build

ENV GO111MODULE="on"
ENV GOOS="linux"
ENV CGO_ENABLED=0
ENV GOGC=70
# runtime builda standalone (sem o workspace), mantendo a imagem enxuta: o
# módulo raiz não depende do tooling em cli/, então GOWORK=off evita baixá-lo.
ENV GOWORK=off

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o /zord cmd/http/main.go

## Deploy
FROM alpine:3.20.2

WORKDIR /

COPY --from=build /zord /

RUN touch .env
RUN touch .config.env

EXPOSE 8080

CMD ["./zord", "http"]
