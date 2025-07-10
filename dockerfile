FROM alpine:3.18

# Add metadata labels
LABEL maintainer="byuoitav"
LABEL description="Mute Service for AV room audio management"
LABEL version="V0.0.4"

RUN apk update && apk add bash && apk --no-cache add tzdata

ARG NAME

COPY ${NAME} /server

ENTRYPOINT ["/server"]