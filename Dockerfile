FROM golang

RUN apt-get update && apt-get install python3-dev python3-pip -y \
    && pip3 install pulp numpy --break-system-packages

WORKDIR /opt/frodo
COPY . .

EXPOSE 8088
CMD ["go", "run", ".", "start", "conf-docker.json"]