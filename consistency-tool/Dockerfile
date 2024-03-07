FROM golang:1.21

RUN mkdir -p /opt/check
WORKDIR /opt/check
COPY . /opt/check

RUN go build -v -o check .

RUN mv /opt/check/check /check
