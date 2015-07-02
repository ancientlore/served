FROM golang

ADD demo.config /go/etc/served.config
ADD . /go/src/github.com/ancientlore/served

RUN go get golang.org/x/tools/present
RUN go get github.com/ancientlore/served/slides
RUN go get golang.org/x/tools/blog
RUN go get golang.org/x/tools/godoc/static
RUN go get golang.org/x/tools/playground
RUN go get golang.org/x/tools/playground/socket
RUN go get golang.org/x/tools/present
RUN go get github.com/kardianos/service

RUN go install github.com/ancientlore/served

ENTRYPOINT ["/go/bin/served", "-run"]

EXPOSE 8000
