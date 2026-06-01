FROM golang:1.25.10 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG APP_VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/AndriGitDev/meil/internal/buildinfo.Version=${APP_VERSION} -X github.com/AndriGitDev/meil/internal/buildinfo.Commit=${GIT_COMMIT} -X github.com/AndriGitDev/meil/internal/buildinfo.BuildTime=${BUILD_TIME}" \
    -o /out/meil ./cmd/meil

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/meil /meil
COPY --from=build /src/migrations /migrations
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/meil"]
CMD ["serve"]
