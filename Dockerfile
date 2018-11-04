FROM golang:1.11-alpine as build

RUN apk --update add git gcc libc-dev

ADD . /go/src/github.com/tolleiv/k8s-affinity-admission
WORKDIR /go/src/github.com/tolleiv/k8s-affinity-admission

RUN go get -u -v github.com/golang/dep/cmd/dep && dep ensure -v
RUN go test
RUN go build -buildmode=pie -ldflags "-linkmode external -extldflags -static -w" -o controller

RUN CGO_ENABLED=0 go build -a -o controller

FROM scratch

USER 1

EXPOSE 8443

COPY --from=build /go/src/github.com/tolleiv/k8s-affinity-admission/controller /

CMD ["/controller","--logtostderr","-v=4","2>&1"]