
.EXPORT_ALL_VARIABLES:
# STANDARD CI/CD
GIT_TAG := $(shell git describe --tags)
BUILD := $(shell git rev-parse --short HEAD)
PROJECTNAME := $(shell basename "$(PWD)")
DOCKERID := nuxion
VERSION := $(shell cat .version | head -n 1)

LDFLAGS=-ldflags "-X=main.Version=$(VERSION) -X=main.Build=$(BUILD)"
STDERR := /tmp/.$(PROJECTNAME)-stderr.txt

.PHONY: run
run:
	go run main.go volume

.PHONY: redis
redis:
	docker-compose exec redis redis-cli

.PHONY: internal-build
internal-build:
	# https://github.com/mattn/go-sqlite3/issues/327
	CGO_ENABLED=1 go build $(LDFLAGS) -o dist/rawdata
	chmod +x dist/rawdata

docker-test:
	docker run --rm -p 6667:6667 ${DOCKERID}/${PROJECTNAME}

## Standard commands for CI/CD cycle

deploy:
	echo "Not implemented"

build-local:
	echo "=> Building ${VERSION} of ${PROJECTNAME}"
	docker build . -t ${DOCKERID}/${PROJECTNAME}
	docker tag ${DOCKERID}/${PROJECTNAME} ${DOCKERID}/${PROJECTNAME}:${VERSION}

build:
	echo "Not implemented"

publish:
	echo "Not implemented"


