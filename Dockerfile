FROM golang:1.20.3-alpine3.16 as builder
# RUN apk add --update --no-cache make git build-base
RUN apk add --update --no-cache make git gcc musl-dev
RUN mkdir /build
ADD . /build/
WORKDIR /build
ENV GOARCH=amd64
ENV CGO_ENABLED=0
ENV GOOS=linux
RUN go mod download
RUN git config --global --add safe.directory /build \
	&& make internal-build

FROM alpine:3.16
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
# TODO -> adduser and binaries to /usr/local/bin path
COPY --from=builder --chown=appuser:appgroup /build/dist/rawdata /usr/local/bin
USER appuser
ENV PATH=$PATH:/usr/local/bin
WORKDIR /app
EXPOSE 6667
RUN chown appuser:appgroup /app

CMD ["rawdata", "volume"]
