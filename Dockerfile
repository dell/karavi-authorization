FROM scratch
LABEL vendor="Dell Inc." \
      name="csm-authorization-sidecar" \
      summary="Dell EMC Container Storage Modules (CSM) for Observability - Metrics for PowerStore" \
      description="CSM for Authorization provides storage and Kubernetes administrators the ability to apply RBAC for Dell EMC CSI Drivers" \
      version="2.0.0" \
      license="Apache-2.0"
ARG APP

WORKDIR /app
COPY $APP /app/command

ENTRYPOINT [ "/app/command" ]
