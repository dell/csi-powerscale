# Includes the following generated file to get semantic version information
#include semver.mk
ifdef NOTES
	RELNOTE="-$(NOTES)"
else
	RELNOTE=
endif

docker:
	docker build -t "artifactory-sio.isus.emc.com:8129/csi-isilon:v1.0.0$(RELNOTE)" .

push:
	docker push "artifactory-sio.isus.emc.com:8129/csi-isilon:v1.0.0$(RELNOTE)"

version:
	@echo "MAJOR $(MAJOR) MINOR $(MINOR) PATCH $(PATCH) BUILD ${BUILD} TYPE ${TYPE} RELNOTE $(RELNOTE) SEMVER $(SEMVER)"
	@echo "Target Version: $(VERSION)"
