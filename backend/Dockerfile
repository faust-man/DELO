FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker && CGO_ENABLED=0 go build -o /out/admin ./cmd/admin
FROM alpine:3.22@sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/ /app/
COPY db /app/db
USER 65532:65532
EXPOSE 8080
CMD ["/app/api"]
