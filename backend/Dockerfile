# syntax=docker/dockerfile:1
FROM golang:1.27.4-alpine@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf AS dependencies
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# A service build contains one entrypoint, never the all-in-one API or admin tools.
FROM dependencies AS service-build
ARG SERVICE
RUN case "$SERVICE" in identity|contacts|organizations|education|bookings|subscriptions|finance|artworks|inventory|exhibitions|reports|audit|notifications|gateway|worker) ;; *) echo 'Unsupported SERVICE' >&2; exit 1 ;; esac \
 && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/service ./cmd/${SERVICE}

FROM alpine:3.22@sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f AS runtime
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
USER 65532:65532
EXPOSE 8080

FROM runtime AS service
ARG SERVICE
LABEL org.opencontainers.image.title="delo-${SERVICE}"
COPY --from=service-build /out/service /app/service
ENTRYPOINT ["/app/service"]

FROM dependencies AS admin-build
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/admin ./cmd/admin
FROM runtime AS admin
COPY --from=admin-build /out/admin /app/admin
COPY db /app/db
ENTRYPOINT ["/app/admin"]

# Compatibility only for the existing Helm deployment; kubectl uses service/admin.
FROM dependencies AS legacy-build
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker && CGO_ENABLED=0 go build -o /out/admin ./cmd/admin
FROM runtime AS legacy
COPY --from=legacy-build /out/ /app/
COPY db /app/db
CMD ["/app/api"]
