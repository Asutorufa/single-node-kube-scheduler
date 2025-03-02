FROM docker.io/library/golang:alpine AS builder

WORKDIR /single-node-kube-scheduler

COPY . .

ENV GOTOOLCHAIN=auto
ENV CGO_ENABLED=0

RUN set -x\
    && apk add --no-cache make git\
    && go build -ldflags='-s -w -buildid=' -v -o single-node-kube-scheduler .

FROM docker.io/library/alpine:latest

WORKDIR /single-node-kube-scheduler

COPY --from=builder /single-node-kube-scheduler/single-node-kube-scheduler /single-node-kube-scheduler/single-node-kube-scheduler

ENTRYPOINT ["/single-node-kube-scheduler/single-node-kube-scheduler"]
