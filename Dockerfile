FROM golang:1.12.4

WORKDIR /go/src/github.com/lightnet328/kubernetes-ssh-container-exposer

ADD . .

RUN go install github.com/lightnet328/kubernetes-ssh-container-exposer

CMD "kubernetes-ssh-container-exposer"
