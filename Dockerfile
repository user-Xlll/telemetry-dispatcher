ARG GOLANG_TAG=1.26.1-se
ARG CURL_TAG=8.9.1@sha256:78c8580bd9480f0d2527c0b781eeb9ffa00f3795f882e625f576aa51af8f4ad5

# ==========================
# Build telemetry-dispatcher
# ==========================

FROM docker-dev.registry-ci.delta.sbrf.ru/ci00757682/ci00757682/golang:${GOLANG_TAG} AS build-telemetry-dispatcher

ENV WORKDIR=/home/builder
ENV CGO_ENABLED=0

USER builder

WORKDIR ${WORKDIR}

COPY --chown=builder:builder ./go.mod go.mod
COPY --chown=builder:builder ./go.sum go.sum
COPY --chown=builder:builder ./main.go main.go
COPY --chown=builder:builder ./vendor vendor
COPY --chown=builder:builder ./cmd cmd

RUN go build -buildvcs=false -o ${WORKDIR}/telemetry-dispatcher -ldflags="-s -w" main.go 

# ====================
# Build otel-collector
# ====================

FROM docker-dev.registry-ci.delta.sbrf.ru/base/docker.io/curlimages/curl:${CURL_TAG} AS build-otel-collector

ARG NEXUS_USERNAME
ARG NEXUS_PASSWORD

ENV WORKDIR=/usr/src
ENV CI=CI05123571
ENV VERSION=0.156.0_Ubuntu_25.04_x86_x64
ENV OUTPUT=distrib.tar.gz
ENV CHECKSUM=${OUTPUT}.sha256

USER root

WORKDIR ${WORKDIR}

RUN curl -u ${NEXUS_USERNAME}:${NEXUS_PASSWORD} -k -w '%{http_code}\n' -o ${WORKDIR}/${CHECKSUM} \
    https://infra.nexus.sigma.sbrf.ru/infra/repository/maven-specsoft/${CI}/${CI}/${VERSION}/${CI}-${VERSION}-${CHECKSUM}

RUN curl -u ${NEXUS_USERNAME}:${NEXUS_PASSWORD} -k -w '%{http_code}\n' -o ${WORKDIR}/${OUTPUT} \
    https://infra.nexus.sigma.sbrf.ru/infra/repository/maven-specsoft/${CI}/${CI}/${VERSION}/${CI}-${VERSION}-${OUTPUT}

RUN echo "$(cat ${CHECKSUM}) ${OUTPUT}" | sha256sum -c \
    && tar -zvxf ${WORKDIR}/${OUTPUT} \
    && mv ${WORKDIR}/otelcol-contrib ${WORKDIR}/otel-collector

# =============
# Runtime stage
# =============

FROM scratch

USER app

COPY --from=build-telemetry-dispatcher /home/builder/telemetry-dispatcher /usr/src/telemetry-dispatcher
COPY --from=build-otel-collector /usr/src/otel-collector /usr/src/otel-collector

ENTRYPOINT ["/usr/src/telemetry-dispatcher"]
