# some arguments that must be supplied
ARG GOPROXY
ARG GOVERSION
ARG BASEIMAGE

# Stage to build the driver
FROM golang:${GOVERSION} as builder
ARG GOPROXY
RUN mkdir -p /go/src
COPY ./ /go/src/
WORKDIR /go/src/
RUN CGO_ENABLED=0 \
    make build

# Stage to build the driver image
FROM $BASEIMAGE AS driver
# install necessary packages
# alphabetical order for easier maintenance
RUN microdnf install -y \
        e4fsprogs \
        libaio \
        libuuid \
        nfs-utils \
        numactl \
        xfsprogs && \
    microdnf clean all
# copy in the driver
COPY --from=builder /go/src/csi-isilon /
ENTRYPOINT ["/csi-isilon"]

# Stage to check for critical and high CVE issues via Trivy (https://github.com/aquasecurity/trivy)
# will break image build if CRITICAL issues found
# will print out all HIGH issues found
FROM driver as cvescan
# run trivy and clean up all traces after
RUN microdnf install -y --enablerepo=ubi-8-baseos tar && \
    microdnf clean all && \
    curl https://raw.githubusercontent.com/aquasecurity/trivy/master/contrib/install.sh | sh && \
    trivy fs -s CRITICAL --exit-code 1 / && \
    trivy fs -s HIGH / && \
    trivy image --reset && \
    rm ./bin/trivy

# final stage
# simple stage to use the driver image as the resultant image
FROM driver as final

LABEL vendor="Dell Inc." \
      name="csi-isilon" \
      summary="CSI Driver for Dell EMC PowerScale" \
      description="CSI Driver for provisioning persistent storage from Dell EMC PowerScale" \
      version="1.4.0" \
      license="Apache-2.0"

COPY ./licenses /licenses
