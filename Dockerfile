FROM golang:1.25.6-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/ws-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /bin/ws-server /ws-server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/ws-server"]
