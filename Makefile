NAME = client

BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
COMMIT = $(shell git rev-parse --short HEAD)
BUILDTIME = $(shell date +%Y-%m-%dT%T%z)
GOPATH = $(shell echo "$$GOPATH")
WD = $(shell pwd)

LD_OPTS = -ldflags="-X main.branch=${BRANCH} -X main.commit=${COMMIT} -X main.lasttag=${LASTTAG} -X main.buildtime=${BUILDTIME} -w "

all:  build run

all-with-deps: setup deps build


run: build
	cd $(WD)/cmd && ./$(NAME) && ../
# run:
# 	./$(NAME)

setup:
	go get -u github.com/kardianos/govendor

deps:
	govendor sync -v

build:
	cd $(WD)/cmd && go build $(LD_OPTS) -o $(NAME) .

# Show to-do items per file.
todo:
	@grep \
		--exclude-dir=vendor \
		--exclude=Makefile \
		--text \
		--color \
		-nRo -E ' TODO:.*|SkipNow|nolint:.*' .
.PHONY: todo

dist:
	GOOS=linux GOARCH=amd64 go build $(LD_OPTS)  -o $(NAME) .

test:
	cd cmd/ && GOOS=linux GOARCH=amd64 go build $(LD_OPTS)  -o test .
