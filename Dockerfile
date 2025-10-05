FROM alpine AS builder

RUN apk add --no-cache \
    curl \
    tar \
    xz \
    gcc \
    musl-dev
ARG MCP_SERVER=
ARG GO_VERSION=1.25.1
RUN curl -L -o go${GO_VERSION}.linux-amd64.tar.gz https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
RUN rm go${GO_VERSION}.linux-amd64.tar.gz

ENV PATH /usr/local/go/bin:$PATH

WORKDIR /app

COPY . .

RUN go build -o main ./${MCP_SERVER}/

FROM alpine

COPY --from=builder /app/main /app/main

CMD ["/app/main"]