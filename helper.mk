# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Dell Technologies, Dell and other trademarks are trademarks of Dell Inc.
# or its subsidiaries. Other trademarks may be trademarks of their respective 
# owners.

generate:
	GOPRIVATE=github.com go generate
	GOPRIVATE=github.com go run core/semver/semver.go -f mk > semver.mk

download-csm-common:
	git clone --depth 1 git@github.com:CSM/csm.git temp-repo
	cp temp-repo/config/csm-common.mk .
	rm -rf temp-repo

vendor:
	GOPRIVATE=github.com go mod vendor
