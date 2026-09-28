# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Dell Technologies, Dell and other trademarks are trademarks of Dell Inc.
# or its subsidiaries. Other trademarks may be trademarks of their respective 
# owners.

.PHONY: generate copy-csm-common vendor

generate:
	GOPRIVATE=github.com go generate
	GOPRIVATE=github.com go run core/semver/semver.go -f mk > semver.mk

copy-csm-common:
	cp ../csm/config/csm-common.mk .

vendor:
	rm -rf vendor
	GOPRIVATE=github.com go mod vendor
