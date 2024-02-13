FROM golang:1.21

RUN mkdir -p /opt/check
WORKDIR /opt/check
COPY . /opt/check

RUN go build -v -o check .

CMD ["./check", "https://172.17.0.1:9000", "minioadmin", "minioadmin", "100", "0"]
