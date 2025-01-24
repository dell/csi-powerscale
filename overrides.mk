# Copyright © 2020-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

# overrides file
# this file, included from the Makefile, will overlay default values with environment variables
#

# DEFAULT values
DEFAULT_GOVERSION="1.21"
DEFAULT_REGISTRY=""
DEFAULT_IMAGENAME="isilon"
DEFAULT_BUILDSTAGE="final"
ifeq ($(origin BUILD_TIMESTAMP), undefined)
BUILD_TIMESTAMP := $(shell date +%Y%m%d%H%M%S)
endif
DEFAULT_IMAGETAG=$(BUILD_TIMESTAMP)
DEFAULT_GOPROXY=""

# set the GOVERSION if needed
ifeq ($(GOVERSION),)
export GOVERSION="$(DEFAULT_GOVERSION)"
endif

# set the REGISTRY if needed
ifeq ($(REGISTRY),)
export REGISTRY="$(DEFAULT_REGISTRY)"
endif

# set the IMAGENAME if needed
ifeq ($(IMAGENAME),)
export IMAGENAME="$(DEFAULT_IMAGENAME)"
endif

# set the IMAGETAG if needed
ifeq ($(IMAGETAG),)
export IMAGETAG="$(DEFAULT_IMAGETAG)"
endif

# set the GOPROXY if needed
ifeq ($(GOPROXY),)
export GOPROXY="$(DEFAULT_GOPROXY)"
endif

# figure out if podman or docker should be used (use podman if found)
ifneq (, $(shell which podman 2>/dev/null))
export BUILDER=podman
else
export BUILDER=docker
endif

# target to print some help regarding these overrides and how to use them
overrides-help:
	@echo
	@echo "The following environment variables can be set to control the build"
	@echo
	@echo "GOVERSION  - The version of Go to build with, default is: $(DEFAULT_GOVERSION)"
	@echo "             Current setting is: $(GOVERSION)"
	@echo "REGISTRY   - The registry to push images to, default is: $(DEFAULT_REGISTRY)"
	@echo "             Current setting is: $(REGISTRY)"
	@echo "IMAGENAME  - The image name to be built, defaut is: $(DEFAULT_IMAGENAME)"
	@echo "             Current setting is: $(IMAGENAME)"
	@echo "IMAGETAG   - The image tag to be built, default is an empty string which will determine the tag by examining annotated tags in the repo."
	@echo "             Current setting is: $(IMAGETAG)"
	@echo "GOPROXY    - The goproxy to be used for resolving dependencies, default is: $(DEFAULT_GOPROXY)"
	@echo "             Current setting is: $(GOPROXY)"
	@echo
