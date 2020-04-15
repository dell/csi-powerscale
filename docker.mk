# Includes the following generated file to get semantic version information
#include semver.mk
ifdef NOTES
	RELNOTE="-$(NOTES)"
else
	RELNOTE=
endif

docker:
	docker build -t "$(REPO_NAME)/$(IMAGE_NAME):$(IMAGE_TAG)$(RELNOTE)" .

push:
	docker push "$(REPO_NAME)/$(IMAGE_NAME):$(IMAGE_TAG)$(RELNOTE)"

version:
	@echo "MAJOR $(MAJOR) MINOR $(MINOR) PATCH $(PATCH) BUILD ${BUILD} TYPE ${TYPE} RELNOTE $(RELNOTE) SEMVER $(SEMVER)"
	@echo "Target Version: $(VERSION)"
