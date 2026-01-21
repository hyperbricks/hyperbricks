FROM golang:1.23.4-alpine

ARG HB_USER=deploy
ARG HB_GROUP=deploy
ARG HB_HOME=/opt/hyperbricks
ARG HB_VERSION=v0.7.6-alpha
ARG TAILWIND_VERSION=4.1.10

ENV HB_HOME=${HB_HOME}
ENV GOPATH=${HB_HOME}/go
ENV GOMODCACHE=${HB_HOME}/go/pkg/mod
ENV GOCACHE=${HB_HOME}/.cache/go-build
ENV GOBIN=${HB_HOME}/bin
ENV PATH=${HB_HOME}/bin:/usr/local/go/bin:${PATH}
ENV HB_DEPLOY_CONFIG=${HB_HOME}/deploy.hyperbricks

RUN apk add --no-cache \
    build-base \
    binutils \
    binutils-gold \
    ca-certificates \
    gcompat \
    git \
    openssh \
    openssh-server \
    openrc \
    tzdata \
    wget

RUN adduser -D -h "${HB_HOME}" "${HB_USER}" \
  && passwd -d "${HB_USER}" \
  && mkdir -p "${HB_HOME}/bin" "${HB_HOME}/deploy" "${HB_HOME}/.ssh" "${HB_HOME}/.cache" "${HB_HOME}/go" \
  && chown -R "${HB_USER}:${HB_GROUP}" "${HB_HOME}"

COPY plugins "${HB_HOME}/plugins"
RUN if [ -d "${HB_HOME}/plugins" ]; then \
      chown -R "${HB_USER}:${HB_GROUP}" "${HB_HOME}/plugins"; \
    fi

USER ${HB_USER}
ENV HOME=${HB_HOME}

WORKDIR ${HB_HOME}
RUN go install "github.com/hyperbricks/hyperbricks/cmd/hyperbricks@${HB_VERSION}"

USER root

RUN install -m 0755 "${HB_HOME}/bin/hyperbricks" /usr/local/bin/hyperbricks

RUN if [ -n "${TAILWIND_VERSION}" ]; then \
      arch="$(apk --print-arch)"; \
      case "$arch" in \
        x86_64) pkgs="tailwindcss-linux-x64-musl tailwindcss-linux-x64" ;; \
        aarch64) pkgs="tailwindcss-linux-arm64-musl tailwindcss-linux-arm64" ;; \
        *) echo "unsupported arch for tailwind: $arch" >&2; exit 1 ;; \
      esac; \
      for pkg in $pkgs; do \
        url="https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAILWIND_VERSION}/${pkg}"; \
        if wget -qO /usr/local/bin/tailwindcss "$url"; then \
          chmod +x /usr/local/bin/tailwindcss; \
          echo "Installed tailwindcss from ${pkg}"; \
          exit 0; \
        fi; \
      done; \
      echo "failed to download tailwindcss for arch ${arch}" >&2; \
      exit 1; \
    fi

RUN mkdir -p /etc/hyperbricks
COPY docker/deploy.hyperbricks /etc/hyperbricks/deploy.hyperbricks
COPY docker/hyperbricks-deploy /etc/init.d/hyperbricks-deploy
COPY docker/hyperbricks-deploy.conf /etc/conf.d/hyperbricks-deploy
COPY docker/sshd_config.conf /etc/ssh/sshd_config.d/99-hyperbricks.conf
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /etc/init.d/hyperbricks-deploy /usr/local/bin/entrypoint.sh

EXPOSE 9090 22

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
