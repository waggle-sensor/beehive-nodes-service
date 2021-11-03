


# docker build -t waggle/beehive-nodes-service .
# docker run -ti --env UPLOADER_URL=http://gateway.docker.internal:8080 --env NODE_STATE_API=... waggle/beehive-nodes-service

FROM golang:1.17-alpine

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["beehive-nodes-service"]