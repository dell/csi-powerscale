# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Dell Technologies, Dell and other trademarks are trademarks of Dell Inc.
# or its subsidiaries. Other trademarks may be trademarks of their respective 
# owners.

include images.mk

# This will be overridden during image build.
IMAGE_VERSION ?= 0.0.0
LDFLAGS = "-X main.ManifestSemver=$(IMAGE_VERSION)"

# default target
all: help

# Help target, prints usefule information
help:
	@echo
	@echo "The following targets are commonly used:"
	@echo
	@echo "build            - Builds the code locally"
	@echo "clean            - Cleans the local build"
	@echo "integration-test - Runs the integration tests. Requires access to an array"
	@echo "unit-test        - Runs the unit tests"
	@echo "vendor 			- Downloads a vendor list (local copy) of repositories required to compile the repo."
	@echo
	@make -s overrides-help

# Clean the build
clean:
	rm -f core/core_generated.go go-code-tester
	rm -f semver.mk
	rm -rf csm-common.mk
	rm -rf vendor
	go clean

format:
	@gofmt -w -s .

# Build the driver locally
build: generate vendor
	CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -mod=vendor -ldflags $(LDFLAGS) -o csi-isilon

# Windows or Linux; requires no hardware
unit-test: go-code-tester
	GITHUB_OUTPUT=/dev/null \
	./go-code-tester 90 "." "" "true" "" "" "./service/mock|./common/constants|./test/integration|./core|./provider"

coverage:
	cd service; go tool cover -html=c.out -o coverage.html

# Linux only; populate env.sh with the hardware parameters
integration-test:
	( cd test/integration; sh run.sh )

gosec:
	gosec -quiet -log gosec.log -out=gosecresults.csv -fmt=csv ./...

go-code-tester:
	git clone --depth 1 git@github.com:CSM/actions.git temp-repo
	cp temp-repo/go-code-tester/entrypoint.sh ./go-code-tester
	chmod +x go-code-tester
	rm -rf temp-repo
