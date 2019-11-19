.PHONY: all
all: build

.PHONY: build
build: docker-build
	docker run --rm -v $(PWD):/work -w /work oslo-tail-build /usr/local/go/bin/go build

.PHONY: docker-build
docker-build:
	docker build -t oslo-tail-build .