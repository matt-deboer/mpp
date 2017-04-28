FROM alpine:3.5
COPY bin/mpp /mpp
COPY ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 9090
ENTRYPOINT ["/mpp"]
