FROM ubuntu:noble

RUN apt-get update && apt-get install git wget tar python3-pip -y \
    && wget https://go.dev/dl/go1.23.6.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz \
    && pip install pulp numpy --break-system-packages

WORKDIR /opt/frodo
COPY . .
RUN git config --global --add safe.directory /opt/frodo \
    && PATH=$PATH:/usr/local/go/bin:/root/go/bin \
    && make swagger && make build

EXPOSE 8777
CMD ["./frodo", "start", "conf.docker.json"]