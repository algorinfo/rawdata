
.EXPORT_ALL_VARIABLES:
VERSION := $(shell git describe --tags)
BUILD := $(shell git rev-parse --short HEAD)
# PROJECTNAME := $(shell basename "$(PWD)")
PROJECTNAME := rawdata

LDFLAGS=-ldflags "-X=main.Version=$(VERSION) -X=main.Build=$(BUILD)"
STDERR := /tmp/.$(PROJECTNAME)-stderr.txt
export CGO_ENABLED=0
# If the first argument is "run"...
ifeq (cert,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "run"
  RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(RUN_ARGS):;@:)
endif

.PHONY: web
web:
	go run main.go web

.PHONY: worker
worker:
	go run main.go worker

.PHONY: build-js
build-js: 
	yarn build


.PHONY: build-go
build-go: 
	go build $(LDFLAGS) -o dist/$(PROJECTNAME)
	chmod +x dist/$(PROJECTNAME)

.PHONY: build
build: build-js build-go
	cp create_db.sh dist/

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
	tar dist.tgz 
