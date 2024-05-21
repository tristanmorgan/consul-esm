FROM golang:alpine AS build

RUN apk update && apk add --no-cache git ca-certificates libcap
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /consul-esm

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /consul-esm /usr/bin/
ENTRYPOINT ["/usr/bin/consul-esm"]
