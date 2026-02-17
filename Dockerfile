# Code Compile
FROM golang:alpine AS build

# ENV GOPROXY=https://goproxy.cn
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 
WORKDIR /app
COPY . .

RUN go build -ldflags="-w -s" -o zpan

# Image Build
FROM alpine:3.21

WORKDIR /app
COPY --from=build /app/zpan /app/zpan

EXPOSE 9000

ENTRYPOINT ["/app/zpan"]
CMD ["server"]
