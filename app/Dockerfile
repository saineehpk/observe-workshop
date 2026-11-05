FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /out/app .

FROM alpine:3.20
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
