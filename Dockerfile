FROM scratch
COPY bin/mpp /mpp
COPY ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/mpp"]