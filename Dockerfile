


# docker build -t waggle/beehive-nodes-service .
# docker run -ti --env-file ./config.env waggle/beehive-nodes-service

FROM golang:1.17-alpine

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["beehive-nodes-service"]