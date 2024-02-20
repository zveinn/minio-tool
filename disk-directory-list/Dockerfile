FROM golang:alpine
WORKDIR /go/src/app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -tags timetzdata -o lister .
ENTRYPOINT ["./lister"]
