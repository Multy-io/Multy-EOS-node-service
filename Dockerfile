FROM golang:1.9.4

RUN go get -u github.com/jekabolt/config && \
    go get -u github.com/jekabolt/slflog && \
    go get -u github.com/eoscanada/eos-go && \
    go get -u github.com/golang/protobuf/proto && \
    go get -u github.com/urfave/cli && \
    go get -u golang.org/x/net/context && \
    go get -u google.golang.org/grpc

RUN mkdir $GOPATH/src/github.com/Multy-io && \
    cd $GOPATH/src/github.com/Multy-io && \ 
    git clone https://github.com/Multy-io/Multy-back.git && \ 
    cd $GOPATH/src/github.com/Multy-io/Multy-back && \ 
    git checkout release_1.1.1 && \  
    git pull origin release_1.1.1

RUN cd $GOPATH/src/github.com/golang/protobuf && \
    make all

RUN apt-get update && \
    apt-get install -y protobuf-compiler

RUN cd $GOPATH/src/github.com/Multy-io && \
    git clone https://github.com/Multy-io/Multy-EOS-node-service.git && \
    cd $GOPATH/src/github.com/Multy-io/Multy-EOS-node-service/ && \
    git checkout master && \
    git pull origin master && \
    make build && \
    rm -r $GOPATH/src/github.com/Multy-io/Multy-back 

WORKDIR $GOPATH/src/github.com/Multy-io/Multy-EOS-node-service/cmd

ENTRYPOINT $GOPATH/src/github.com/Multy-io/Multy-EOS-node-service/cmd/client
