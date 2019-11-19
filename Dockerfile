FROM centos:7

RUN \
    yum install -y wget gcc libpcap-devel && \
    wget https://dl.google.com/go/go1.13.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.13.linux-amd64.tar.gz