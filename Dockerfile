ARG GOPROXY

FROM centos:8
ARG GOPROXY
RUN yum install -y libaio
RUN yum install -y libuuid
RUN yum install -y numactl
RUN yum install -y xfsprogs
RUN yum install -y e4fsprogs
RUN yum install -y nfs-utils
RUN yum --enablerepo=cr update -y
RUN yum clean all
COPY "csi-isilon" .
ENTRYPOINT ["/csi-isilon"]
