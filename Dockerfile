# Copyright © 2019-2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

ARG GOPROXY

FROM rockylinux:9
ARG GOPROXY

RUN yum install -y libaio libuuid numactl xfsprogs e4fsprogs nfs-utils

RUN yum clean all
COPY "csi-isilon" .
ENTRYPOINT ["/csi-isilon"]
