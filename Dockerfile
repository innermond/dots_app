FROM --platform=linux/amd64 golang:1.20
RUN \
    cd /tmp && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    cp /go/bin/dlv /usr/bin/dlv
COPY ./files/ /
RUN \
    mkdir /.cache && chmod 777 /.cache; exit 0
