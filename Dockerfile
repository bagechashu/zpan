# Code Compile
FROM golang:alpine AS build

# ENV GOPROXY=https://goproxy.cn
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64 
WORKDIR /app

# Install build dependencies required for cgo (sqlite3)
RUN apk add --no-cache gcc musl-dev

COPY . .

RUN go build -ldflags="-w -s" -o zpan

# Image Build
FROM alpine:3.21

WORKDIR /app
COPY --from=build /app/zpan /app/zpan

EXPOSE 9000

ENTRYPOINT ["/app/zpan"]
CMD ["server"]
