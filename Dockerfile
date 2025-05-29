FROM golang

RUN apt-get update && apt-get install python3-dev python3-pip -y \
    && pip3 install pulp numpy --break-system-packages

WORKDIR /opt/frodo
COPY . .

RUN git config --global --add safe.directory /opt/frodo && make build

EXPOSE 8088
CMD ["./frodo", "start", "conf-docker.json"]