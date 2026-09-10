FROM golang:1.26.1-alpine

ARG HB_USER=deploy
ARG HB_GROUP=deploy
ARG HB_HOME=/opt/hyperbricks
ARG HB_VERSION=v2.0.0-beta
ARG HB_BUILD_SOURCE=checkout
ARG TAILWIND_VERSION=

ENV HB_HOME=${HB_HOME}
ENV HB_USER=${HB_USER}
ENV HB_GROUP=${HB_GROUP}
ENV GOPATH=${HB_HOME}/go
ENV GOMODCACHE=${HB_HOME}/go/pkg/mod
ENV GOCACHE=${HB_HOME}/.cache/go-build
ENV GOBIN=${HB_HOME}/bin
ENV PATH=${HB_HOME}/bin:/usr/local/go/bin:${PATH}
ENV HB_DEPLOY_CONFIG=${HB_HOME}/deploy.hyperbricks.yaml

RUN apk add --no-cache \
    build-base \
    binutils \
    binutils-gold \
    ca-certificates \
    gcompat \
    git \
    su-exec \
    tzdata \
    wget

RUN adduser -D -h "${HB_HOME}" "${HB_USER}" \
  && passwd -d "${HB_USER}" \
  && mkdir -p "${HB_HOME}/bin" "${HB_HOME}/deploy" "${HB_HOME}/.cache" "${HB_HOME}/go" \
  && chown -R "${HB_USER}:${HB_GROUP}" "${HB_HOME}"

COPY go.mod go.sum /opt/hyperbricks-source/
COPY assets /opt/hyperbricks-source/assets
COPY cmd /opt/hyperbricks-source/cmd
COPY pkg /opt/hyperbricks-source/pkg
RUN chown -R "${HB_USER}:${HB_GROUP}" /opt/hyperbricks-source

COPY plugins "${HB_HOME}/plugins"
RUN if [ -d "${HB_HOME}/plugins" ]; then \
      chown -R "${HB_USER}:${HB_GROUP}" "${HB_HOME}/plugins"; \
    fi

USER ${HB_USER}
ENV HOME=${HB_HOME}

WORKDIR ${HB_HOME}
RUN case "${HB_BUILD_SOURCE}" in \
      checkout) cd /opt/hyperbricks-source && go install ./cmd/hyperbricks ;; \
      release) go install "github.com/hyperbricks/hyperbricks/cmd/hyperbricks@${HB_VERSION}" ;; \
      *) echo "HB_BUILD_SOURCE must be checkout or release" >&2; exit 1 ;; \
    esac
ENV HB_BUILD_SOURCE=${HB_BUILD_SOURCE}

USER root

RUN install -m 0755 "${HB_HOME}/bin/hyperbricks" /usr/local/bin/hyperbricks

RUN if [ -n "${TAILWIND_VERSION}" ]; then \
      arch="$(apk --print-arch)"; \
      case "$arch" in \
        x86_64) pkgs="tailwindcss-linux-x64-musl tailwindcss-linux-x64" ;; \
        aarch64) pkgs="tailwindcss-linux-arm64-musl tailwindcss-linux-arm64" ;; \
        *) echo "unsupported arch for tailwind: $arch" >&2; exit 1 ;; \
      esac; \
      release_url="https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAILWIND_VERSION}"; \
      wget -qO /tmp/tailwindcss-sha256sums.txt "${release_url}/sha256sums.txt"; \
      for pkg in $pkgs; do \
        if wget -qO /tmp/tailwindcss "${release_url}/${pkg}"; then \
          checksum="$(awk -v name="./${pkg}" '$2 == name { print $1 }' /tmp/tailwindcss-sha256sums.txt)"; \
          test -n "$checksum" || { echo "no published checksum for ${pkg}" >&2; exit 1; }; \
          echo "${checksum}  /tmp/tailwindcss" | sha256sum -c -; \
          install -m 0755 /tmp/tailwindcss /usr/local/bin/tailwindcss; \
          rm -f /tmp/tailwindcss /tmp/tailwindcss-sha256sums.txt; \
          echo "Installed tailwindcss from ${pkg}"; \
          exit 0; \
        fi; \
      done; \
      echo "failed to download tailwindcss for arch ${arch}" >&2; \
      exit 1; \
    fi

RUN mkdir -p /etc/hyperbricks
COPY docker/deploy.hyperbricks.yaml /etc/hyperbricks/deploy.hyperbricks.yaml
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 9090

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
