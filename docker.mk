# docker makefile, included from Makefile, will build/push images with docker or podman

docker-build:
	@echo "Building docker image: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	docker build -t "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)" --build-arg GOPROXY=$(GOPROXY) --build-arg GOVERSION=$(GOVERSION) .

docker-build-image-push:
	@echo "Pushing: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	docker push "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"

podman-build: download-csm-common
	$(eval include csm-common.mk)
	@echo "Base Image is set to: $(DEFAULT_BASEIMAGE)"
	@echo "Adding Driver dependencies to $(DEFAULT_BASEIMAGE)"
	bash ./buildubimicro.sh $(DEFAULT_BASEIMAGE)
	@echo "Base image build: SUCCESS" $(eval BASEIMAGE=localhost/csipowerscale-ubimicro:latest)
	@echo "Building: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	@echo "Using Golang Image $(DEFAULT_GOIMAGE)"
	$(BUILDER) build -t "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)" -f Dockerfile.podman --target $(BUILDSTAGE) --build-arg GOPROXY=$(GOPROXY) --build-arg BASEIMAGE=$(BASEIMAGE) --build-arg GOIMAGE=$(DEFAULT_GOIMAGE) .

podman-build-image-push:
	@echo "Pushing: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	$(BUILDER) push "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"

version:
	@echo "MAJOR $(MAJOR) MINOR $(MINOR) PATCH $(PATCH) BUILD ${BUILD} TYPE ${TYPE} RELNOTE $(RELNOTE) SEMVER $(SEMVER)"
	@echo "Target Version: $(VERSION)"

download-csm-common:
	curl -O -L https://raw.githubusercontent.com/dell/csm/main/config/csm-common.mk