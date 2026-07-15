FROM golang:1.26-trixie

ENV DEBIAN_FRONTEND=noninteractive
ENV container=docker
ENV SNCLIENT_TEST_SETUP=container

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    libcap2-bin \
    ca-certificates \
    curl \
    git \
    logrotate \
    make \
    openssl \
    sudo \
    dbus \
    systemd \
  && rm -rf /var/lib/apt/lists/*
RUN rm -f /usr/sbin/policy-rc.d
COPY ./debian/sudoers /etc/sudoers.d/snclient
COPY ./debian/test_id.sh /usr/local/bin/test_id.sh
COPY ./debian/snclient_local_docker.ini /var/tmp/snclient_local_docker.ini

WORKDIR /src

STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]
