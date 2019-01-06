# (cd deployments/build_runner && docker build -t golangci/build-runner .)
FROM golang:1.11 as builder

ENV GOPATH=/go
ENV GOBINPATH=$GOPATH/bin

WORKDIR ${GOPATH}

ENV DEP_RELEASE_TAG=v0.5.0
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

ENV GLIDE_RELEASE_TAG=v0.13.2
RUN (wget -O - https://github.com/Masterminds/glide/releases/download/${GLIDE_RELEASE_TAG}/glide-${GLIDE_RELEASE_TAG}-linux-amd64.tar.gz | tar -zxvf -) && \
    mv linux-amd64/glide ${GOBINPATH}/

ENV GOVENDOR_VERSION=v1.0.8
RUN wget https://github.com/kardianos/govendor/releases/download/${GOVENDOR_VERSION}/govendor_linux_amd64 -O $GOBINPATH/govendor && \
    chmod a+x $GOBINPATH/govendor

ENV GODEP_VERSION=v80
RUN wget https://github.com/tools/godep/releases/download/${GODEP_VERSION}/godep_linux_amd64 -O $GOBINPATH/godep && \
    chmod a+x $GOBINPATH/godep

RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b $GOPATH/bin v1.12.3

WORKDIR ${GOPATH}/src/github.com/golangci/golangci-api
RUN git clone https://github.com/golangci/golangci-api.git . && \
    git checkout 6ca91c7c462a4441397c61e5cb71949d69622e3f && \
    GO111MODULE=on go mod vendor && \
    go install ./cmd/buildrunner && \
    go install ./cmd/goenvbuild

FROM golang:1.11

ENV GOPATH=/go
ENV GOBINPATH=$GOPATH/bin
ENV PATH=$PATH:/usr/local/go/bin:$GOBINPATH

COPY --from=builder ${GOPATH}/bin/* ${GOPATH}/bin/

WORKDIR /goapp

ENV PORT=7000 MAX_LIFETIME=30m

CMD ["buildrunner"]