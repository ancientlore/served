FROM golang:alpine

RUN apk add --update git subversion mercurial && rm -rf /var/cache/apk/*

ADD demo.config /go/etc/served.config
ADD . /go/src/github.com/ancientlore/served

WORKDIR /go/src/github.com/ancientlore/served

RUN go get
RUN go install

WORKDIR /go

ENTRYPOINT ["/go/bin/served", "-run"]

EXPOSE 8000
