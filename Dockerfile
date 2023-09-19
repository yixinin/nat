FROM alpine:3.18

ADD ./nat /app/nat

WORKDIR /app

CMD ["./nat"]