# docker makefile, included from Makefile, will build/push images with docker or podman

docker-build:
	@echo "Building docker image: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	docker build -t "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)" --build-arg GOPROXY=$(GOPROXY) --build-arg GOVERSION=$(GOVERSION) .

docker-build-image-push:
	@echo "Pushing: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	docker push "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"

podman-build:
	@echo "Building: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	$(BUILDER) build -t "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)" -f Dockerfile.podman --target $(BUILDSTAGE) --build-arg GOPROXY=$(GOPROXY) --build-arg BASEIMAGE=$(BASEIMAGE) --build-arg GOVERSION=$(GOVERSION) .

podman-build-image-push:
	@echo "Pushing: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	$(BUILDER) push "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"

version:
	@echo "MAJOR $(MAJOR) MINOR $(MINOR) PATCH $(PATCH) BUILD ${BUILD} TYPE ${TYPE} RELNOTE $(RELNOTE) SEMVER $(SEMVER)"
	@echo "Target Version: $(VERSION)"
