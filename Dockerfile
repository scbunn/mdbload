FROM golang as builder
LABEL maintainer="Stephen Bunn"
ARG GIT_SHA
ARG VERSION
ARG BUILD_TIME

WORKDIR /go/src/github.com/scbunn/mdbload
COPY . .
RUN go get .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app -ldflags "-s -w -X main.BUILD_DATE=$BUILD_TIME -X main.GIT_SHA=$GIT_SHA -X main.VERSION=$VERSION" .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

WORKDIR /bin
COPY --from=builder /go/src/github.com/scbunn/mdbload/app .
COPY templates/ /etc/mdbload
ENTRYPOINT ["./app"]
CMD ["--help"]
