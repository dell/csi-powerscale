# Copyright © 2021-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

# some arguments that must be supplied
ARG GOIMAGE
ARG BASEIMAGE
ARG VERSION="2.16.0"

# Stage to build the driver
FROM $GOIMAGE as builder
ARG VERSION

RUN mkdir -p /go/src
COPY ./ /go/src/

WORKDIR /go/src/
RUN make build IMAGE_VERSION=$VERSION

# Stage to build the driver image
FROM $BASEIMAGE AS final
ARG VERSION

# copy in the driver
COPY --from=builder /go/src/csi-isilon /
ENTRYPOINT ["/csi-isilon"]

LABEL vendor="Dell Technologies" \
    maintainer="Dell Technologies" \
    name="csi-isilon" \
    summary="CSI Driver for Dell EMC PowerScale" \
    description="CSI Driver for provisioning persistent storage from Dell EMC PowerScale" \
    release="1.16.0" \
    version=$VERSION \
    license="Apache-2.0"

COPY ./licenses /licenses
