FROM golang:1.22.3@sha256:f43c6f049f04cbbaeb28f0aad3eea15274a7d0a7899a617d0037aec48d7ab010 AS builder

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -y -qq update \
    && apt-get -y -qq install "make"

ADD . /go/src/github.com/telia-oss/github-pr-resource
WORKDIR /go/src/github.com/telia-oss/github-pr-resource

RUN go version \
    && make all



FROM alpine:3.20.0@sha256:77726ef6b57ddf65bb551896826ec38bc3e53f75cdde31354fbffb4f25238ebd AS resource
RUN apk add --update --no-cache \
    git \
    git-lfs \
    openssh \
    git-crypt
COPY scripts/askpass.sh /usr/local/bin/askpass.sh
COPY --from=builder /go/src/github.com/telia-oss/github-pr-resource/build /opt/resource
RUN chmod +x /opt/resource/*



FROM resource
LABEL MAINTAINER=cloudfoundry-community
