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
	rm -f core/core_generated.go go-code-tester
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

docker: build
	make -f docker.mk docker

# Pushes container to the repository
podman-build-image-push: podman-build
	make -f docker.mk podman-build-image-push

dev-build-image-push: dev-build
	make -f docker.mk docker-build-image-push

# Windows or Linux; requires no hardware
unit-test: go-code-tester
	GITHUB_OUTPUT=/dev/null \
	./go-code-tester 90 "." "" "true" "" "" "./service/mock|./common/constants|./test/integration|./core|./provider"
	( cd service; go test -race -coverprofile=c.out ./... )

coverage:
	cd service; go tool cover -html=c.out -o coverage.html

# Linux only; populate env.sh with the hardware parameters
integration-test:
	( cd test/integration; sh run.sh )

version:
	go generate
	go run core/semver/semver.go -f mk >semver.mk
	make -f docker.mk version

gosec:
	gosec -quiet -log gosec.log -out=gosecresults.csv -fmt=csv ./...

.PHONY: actions action-help
actions: ## Run all GitHub Action checks that run on a pull request creation
	@echo "Running all GitHub Action checks for pull request events..."
	@act -l | grep -v ^Stage | grep pull_request | grep -v image_security_scan | awk '{print $$2}' | while read WF; do \
		echo "Running workflow: $${WF}"; \
		act pull_request --no-cache-server --platform ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-latest --job "$${WF}"; \
	done

go-code-tester:
	curl -o go-code-tester -L https://raw.githubusercontent.com/dell/common-github-actions/main/go-code-tester/entrypoint.sh \
	&& chmod +x go-code-tester

action-help: ## Echo instructions to run one specific workflow locally
	@echo "GitHub Workflows can be run locally with the following command:"
	@echo "act pull_request --no-cache-server --platform ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-latest --job <jobid>"
	@echo ""
	@echo "Where '<jobid>' is a Job ID returned by the command:"
	@echo "act -l"
	@echo ""
	@echo "NOTE: if act is not installed, it can be downloaded from https://github.com/nektos/act"
