# docker makefile, included from Makefile, will build/push images with docker or podman

docker: download-csm-common
	$(eval include csm-common.mk)
	@echo "Building docker image: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	docker build --pull -t "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)" --build-arg GOPROXY=$(GOPROXY) --build-arg BASEIMAGE=$(CSM_BASEIMAGE) --build-arg GOIMAGE=$(DEFAULT_GOIMAGE) .

docker-push:
	@echo "Pushing: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	docker push "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"

podman-build: download-csm-common
	$(eval include csm-common.mk)
	@echo "Building: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	@echo "Using Golang Image $(DEFAULT_GOIMAGE)"
	$(BUILDER) build --pull $(NOCACHE) -t "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)" --build-arg GOPROXY=$(GOPROXY) --build-arg BASEIMAGE=$(CSM_BASEIMAGE) --build-arg GOIMAGE=$(DEFAULT_GOIMAGE) .

podman-build-no-cache:
	@echo "Building with --no-cache ..."
	@make podman-build NOCACHE=--no-cache

podman-build-image-push:
	@echo "Pushing: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	$(BUILDER) push "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"

version:
	@echo "MAJOR $(MAJOR) MINOR $(MINOR) PATCH $(PATCH) BUILD ${BUILD} TYPE ${TYPE} RELNOTE $(RELNOTE) SEMVER $(SEMVER)"
	@echo "Target Version: $(VERSION)"

download-csm-common:
	curl -O -L https://raw.githubusercontent.com/dell/csm/main/config/csm-common.mk