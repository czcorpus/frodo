FROM ubuntu:noble

RUN apt-get update && apt-get install git wget curl tar python3-pip -y \
    && wget https://go.dev/dl/go1.23.6.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz \
    && pip install pulp numpy --break-system-packages

ENV PATH="${PATH}:/usr/local/go/bin:/root/go/bin"

ARG INSTALL_AIR=false
RUN if [ "$INSTALL_AIR" = "true" ]; then \
        curl -sSfL https://raw.githubusercontent.com/air-verse/air/master/install.sh | sh -s -- -b /usr/local/bin \
        && /usr/local/bin/air -v; \
    fi

WORKDIR /opt/frodo
COPY . .
RUN git config --global --add safe.directory /opt/frodo \
    && make build

CMD ["./frodo", "start", "conf.docker.json"]