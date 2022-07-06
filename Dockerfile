FROM       golang:1.18.3 AS build-env
ENV        WORKDIR_PATH /go/src/github.com/kazz187/fargate-td
ADD        . ${WORKDIR_PATH}
WORKDIR    ${WORKDIR_PATH}/cmd/fargate-td
RUN        CGO_ENABLED=0 go build -o /bin/ && strip /bin/fargate-td

FROM       alpine:latest
COPY       --from=build-env /bin/fargate-td /usr/local/bin/fargate-td
CMD        [ "/usr/local/bin/fargate-td" ]
