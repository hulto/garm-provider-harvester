FROM docker.io/golang:alpine AS build

WORKDIR /root
USER root

RUN apk add musl-dev gcc libtool m4 autoconf g++ make libblkid util-linux-dev git linux-headers mingw-w64-gcc

RUN wget http://musl.cc/aarch64-linux-musl-cross.tgz -O /tmp/aarch64-linux-musl-cross.tgz && \
    tar --strip-components=1 -C /usr/local -xzf /tmp/aarch64-linux-musl-cross.tgz && \
    rm /tmp/aarch64-linux-musl-cross.tgz

ADD ./scripts/build-static.sh /build-static.sh
RUN chmod +x /build-static.sh

CMD ["/bin/sh"]

FROM ghcr.io/cloudbase/garm:nightly AS release
ARG rev
COPY build/$rev/linux/amd64/garm-provider-harvester /opt/garm/providers.d/garm-provider-harvester
RUN chmod 755 /opt/garm/providers.d/garm-provider-harvester

ENTRYPOINT ["/bin/garm", "-config", "/etc/garm/config.toml"]