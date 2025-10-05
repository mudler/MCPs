FROM golang:1.25-alpine AS builder

ARG MCP_SERVER=

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build -o main ./${MCP_SERVER}/

FROM alpine

COPY --from=builder /app/main /app/main

CMD ["/app/main"]