

FROM golang:1.20 AS builder

ARG GH_USER_NAME
ARG GH_BUILD_TOKEN

###################### DATABASE SETUP #######################

WORKDIR /src

# Copty go mod and sum
ADD src/go.mod /src

# Download dependencies
RUN mkdir -p /database \
    && git config --global url."https://${GH_USER_NAME}:${GH_BUILD_TOKEN}@github.com".insteadOf "https://github.com" \
    && export GOPRIVATE="github.com/gage-technologies" \
    && /usr/local/go/bin/go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs /usr/local/go/bin/go get

# Copy project files
ADD src/. /src

RUN /usr/local/go/bin/go generate && /usr/local/go/bin/go build -o /tmp/gigo-core .

############################################################

FROM golang:1.20

################### INSTALL TIDB-TOOLKIT #####################

WORKDIR /

RUN cd /tmp && wget https://download.pingcap.org/tidb-community-toolkit-v6.1.2-linux-amd64.tar.gz \
    && tar -xvf tidb-community-toolkit-v6.1.2-linux-amd64.tar.gz \
    && rm tidb-community-toolkit-v6.1.2-linux-amd64.tar.gz \
    && cd tidb-community-toolkit-v6.1.2-linux-amd64  \
    && tar -xvf br-v6.1.2-linux-amd64.tar.gz \
    && mv br /usr/local/bin/ \
    && cd .. \
    && rm -rf tidb-community-toolkit-v6.1.2-linux-amd64

COPY --from=builder /tmp/gigo-core /bin

RUN mkdir -p /logs \
    && mkdir -p /keys \
    && mkdir -p /gigo-core \
    && mkdir -p /db-files

ENV NAME GIGO
ENV TZ "America/Chicago"
ENTRYPOINT ["/bin/gigo-core"]
