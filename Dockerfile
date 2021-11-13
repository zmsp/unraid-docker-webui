FROM alpine:latest as builder

RUN apk update && apk add go

WORKDIR /app

ADD . .

RUN go build

FROM alpine:latest

MAINTAINER "Samuel MICHAUX <samuel.michaux@gmail.com>"

RUN mkdir /app

COPY --from=builder /app/unraid-docker-webui /app

RUN mkdir /data
VOLUME ["/data"]

WORKDIR /app

ENV CIRCLE "no"

EXPOSE 8080/tcp

CMD ["./unraid-docker-webui"]