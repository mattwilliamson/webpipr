FROM golang
MAINTAINER matt@aimatt.com

ADD . /go/src/github.com/mattwilliamson/webpipr

RUN go install github.com/mattwilliamson/webpipr

ENTRYPOINT /go/bin/webpipr

EXPOSE 8080