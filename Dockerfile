FROM golang:1.11
WORKDIR /go/src/github.com/aau-network-security/haaukins
COPY . .

RUN go get -d -v ./...