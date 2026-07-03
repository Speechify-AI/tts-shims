# syntax=docker/dockerfile:1

# PROVIDER selects which cmd/<PROVIDER> binary to build, e.g.
#   docker build --build-arg PROVIDER=openai -t speechify-ai/openai-shim .
ARG PROVIDER=openai

FROM golang:1.26-alpine AS build
ARG PROVIDER
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/shim ./cmd/${PROVIDER}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/shim /shim
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/shim"]
