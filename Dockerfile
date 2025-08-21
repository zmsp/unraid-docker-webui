FROM alpine:latest as builder

RUN apk update && apk add go

WORKDIR /app

ADD . .

RUN go build

FROM alpine:latest

MAINTAINER "Zobair <zprimellc@gmail.com>"

RUN mkdir /app

COPY --from=builder /app/unraid-docker-webui /app

RUN mkdir -p /data
VOLUME ["/data"]

RUN mkdir -p /config
VOLUME ["/config"]

WORKDIR /app

ENV CIRCLE "no"

ENV PORT 80

EXPOSE 8097/tcp

CMD ["./unraid-docker-webui"]