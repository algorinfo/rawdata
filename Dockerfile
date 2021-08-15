FROM golang:1-alpine as builder
# RUN apk add --update --no-cache make git build-base
RUN apk add --update --no-cache make git gcc musl-dev
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go mod download
RUN make build
ENV GOARCH=amd64
ENV CGO_ENABLED=0
ENV GOOS=linux

FROM alpine
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
# TODO -> adduser and binaries to /usr/local/bin path
COPY --from=builder --chown=appuser:appgroup /build/ /app/
USER appuser
ENV PATH=$PATH:/app/dist
#EXPOSE 5443/tcp
WORKDIR /app
#ENTRYPOINT ["/app/bin/"]
EXPOSE 6667
CMD ["/app/dist/rawdata", "volume"]
