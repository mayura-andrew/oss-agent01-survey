# Build a small static binary, then run it in a minimal, non-root image.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY web ./web
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /survey .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /survey /survey
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/survey"]
