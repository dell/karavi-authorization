FROM scratch

ARG APP

WORKDIR /app
COPY $APP /app/command

ENTRYPOINT [ "/app/command" ]