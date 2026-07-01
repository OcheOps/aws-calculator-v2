# syntax=docker/dockerfile:1.7
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/aws-calc-api ./cmd/aws-calc-api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/aws-calc-api /aws-calc-api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/aws-calc-api"]
