
.EXPORT_ALL_VARIABLES:
VERSION := $(shell git describe --tags)
BUILD := $(shell git rev-parse --short HEAD)
# PROJECTNAME := $(shell basename "$(PWD)")
PROJECTNAME := rawdata

LDFLAGS=-ldflags "-X=main.Version=$(VERSION) -X=main.Build=$(BUILD)"
STDERR := /tmp/.$(PROJECTNAME)-stderr.txt

.PHONY: run
run:
	go run main.go volume

.PHONY: redis
redis:
	docker-compose exec redis redis-cli

.PHONY: build
build:
	# https://github.com/mattn/go-sqlite3/issues/327
	CGO_ENABLED=1 go build $(LDFLAGS) -o dist/$(PROJECTNAME)
	chmod +x dist/$(PROJECTNAME)

.PHONY: tar
tar:
	tar cvfz releases/$(PROJECTNAME)-$(VERSION).tgz dist/

.PHONY: docker
docker: 
	docker build -t nuxion/${PROJECTNAME} .

.PHONY: release
release: docker
	docker tag nuxion/${PROJECTNAME} nuxion/${PROJECTNAME}:$(VERSION)
	docker push nuxion/$(PROJECTNAME):$(VERSION)

.PHONY: migrate
migrate:
	migrate -database ${AGW_DB} -path migrations/ up

.PHONY: tag
tag:
	#poetry version prealese
	git tag -a $(shell poetry version --short) -m "$(shell git log -1 --pretty=%B | head -n 1)"

