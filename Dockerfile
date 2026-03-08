FROM alpine:latest

RUN apk add --no-cache inotify-tools curl jq

COPY pluck.sh /usr/local/bin/pluck
RUN chmod +x /usr/local/bin/pluck

COPY rules.example /etc/pluck/rules

ENTRYPOINT ["pluck"]
