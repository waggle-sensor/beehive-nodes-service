
#export NODE_STATE_API="https://api.sagecontinuum.org/api/state"

# docker build -t waggle/beehive-nodes-service .
# docker run -ti -v ~/git/honeyhouse-config/applications/beehive-sage/beehive-nodes-service/beehive-master.cert:/etc/tls/cert.pem:ro -v ~/git/honeyhouse-config/applications/beehive-sage/beehive-nodes-service/beehive-master.pem:/etc/tls/key.pem:ro --env NODE_STATE_API=${NODE_STATE_API} waggle/beehive-nodes-service

FROM golang:1.17-alpine

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["beehive-nodes-service"]