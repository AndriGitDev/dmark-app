FROM golang:1.25.10 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/meil ./cmd/meil

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/meil /meil
COPY --from=build /src/migrations /migrations
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/meil"]
CMD ["serve"]
