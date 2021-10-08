# overrides file
# this file, included from the Makefile, will overlay default values with environment variables
#

# DEFAULT values
# ubi8/ubi-minimal:8.4-210
DEFAULT_BASEIMAGE="registry.access.redhat.com/ubi8/ubi-minimal@sha256:27a3683c44e97432453ad25bf4a64d040e2c82cfe0e0b36ebf1d74d68e22b1c6"
DEFAULT_GOVERSION="1.16.3"
DEFAULT_REGISTRY=""
DEFAULT_IMAGENAME="isilon"
DEFAULT_BUILDSTAGE="final"
ifeq ($(origin BUILD_TIMESTAMP), undefined)
BUILD_TIMESTAMP := $(shell date +%Y%m%d%H%M%S)
endif
DEFAULT_IMAGETAG=$(BUILD_TIMESTAMP)
DEFAULT_GOPROXY=""

# set the BASEIMAGE if needed
ifeq ($(BASEIMAGE),)
export BASEIMAGE="$(DEFAULT_BASEIMAGE)"
endif

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

# set the BUILDSTAGE if needed
ifeq ($(BUILDSTAGE),)
export BUILDSTAGE="$(DEFAULT_BUILDSTAGE)"
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
	@echo "BASEIMAGE  - The base container image to build from, default is: $(DEFAULT_BASEIMAGE)"
	@echo "             Current setting is: $(BASEIMAGE)"
	@echo "REGISTRY   - The registry to push images to, default is: $(DEFAULT_REGISTRY)"
	@echo "             Current setting is: $(REGISTRY)"
	@echo "IMAGENAME  - The image name to be built, defaut is: $(DEFAULT_IMAGENAME)"
	@echo "             Current setting is: $(IMAGENAME)"
	@echo "IMAGETAG   - The image tag to be built, default is an empty string which will determine the tag by examining annotated tags in the repo."
	@echo "             Current setting is: $(IMAGETAG)"
	@echo "BUILDSTAGE - The Dockerfile build stage to execute, default is: $(DEFAULT_BUILDSTAGE)"
	@echo "             Stages can be found by looking at the Dockerfile"
	@echo "             Current setting is: $(BUILDSTAGE)"
	@echo "GOPROXY    - The goproxy to be used for resolving dependencies, default is: $(DEFAULT_GOPROXY)"
	@echo "             Current setting is: $(GOPROXY)"
	@echo
