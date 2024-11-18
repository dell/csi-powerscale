# default target
all: help

# include an overrides file, which sets up default values and allows user overrides
include overrides.mk

# Help target, prints usefule information
help:
	@echo
	@echo "The following targets are commonly used:"
	@echo
	@echo "build            - Builds the code locally"
	@echo "clean            - Cleans the local build"
	@echo "docker           - Builds the code within a golang container and then creates the driver image"
	@echo "integration-test - Runs the integration tests. Requires access to an array"
	@echo "push             - Pushes the built container to a target registry"
	@echo "unit-test        - Runs the unit tests"
	@echo
	@make -s overrides-help

# Clean the build
clean:
	rm -f core/core_generated.go
	rm -f semver.mk
	go clean

# Dependencies
dependencies:
	go generate
	go run core/semver/semver.go -f mk >semver.mk

format:
	@gofmt -w -s .

# Build the driver locally
build: dependencies
	GOOS=linux CGO_ENABLED=0 go build

# Generates the docker container (but does not push)
podman-build:
	make -f docker.mk podman-build

# Generates the docker container without using cache(but does not push)
podman-build-no-cache:
	make -f docker.mk podman-build-no-cache

dev-build: build
	make -f docker.mk docker-build
	
# Pushes container to the repository
podman-build-image-push: podman-build
	make -f docker.mk podman-build-image-push

dev-build-image-push: dev-build
	make -f docker.mk docker-build-image-push

# Windows or Linux; requires no hardware
unit-test:
	( cd service; go clean -cache; go test -v -coverprofile=c.out ./... )

# Linux only; populate env.sh with the hardware parameters
integration-test:
	( cd test/integration; sh run.sh )

version:
	go generate
	go run core/semver/semver.go -f mk >semver.mk
	make -f docker.mk version

gosec:
	gosec -quiet -log gosec.log -out=gosecresults.csv -fmt=csv ./...

