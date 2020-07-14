FROM golang:1.14 as builder
WORKDIR /go/src/github.com/ancientlore/served
COPY . .
RUN go version
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on go get .
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on go install

FROM gcr.io/distroless/static:nonroot
COPY demo.config /go/etc/served.config
COPY . /go/src/github.com/ancientlore/served
COPY --from=builder /go/bin/served /go/bin/served
EXPOSE 8000
ENTRYPOINT ["/go/bin/served", "-run"]
