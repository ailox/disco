FROM docker.io/library/golang:1.22.7 AS builder
WORKDIR /app/
COPY . /app
ARG GOARCH=amd64
ARG GOARM=7
ARG VERSION=devbuild
ARG REVISION=0000000

RUN \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOPROXY=https://proxy.golang.org,direct \
    go build \
        -trimpath -buildvcs=false \
        -ldflags "-buildid= -s -w -X main.Version=$VERSION -X main.Revision=$REVISION" \
        -o main .


FROM scratch
EXPOSE 8080/tcp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/main main
ENTRYPOINT ["/main"]
CMD ["server"]
