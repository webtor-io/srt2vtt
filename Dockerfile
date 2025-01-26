FROM golang:1.23.5-alpine3.21 AS build

RUN apk add --no-cache build-base

# set work dir
WORKDIR /app

# copy the source files
COPY . .

ENV GOOS=linux CGO_LDFLAGS="-static" CGO_ENABLED=1

# build the binary with debug information removed
RUN go build -ldflags '-w -s' -a -installsuffix cgo -o server

FROM alpine:3.21

# copy our static linked library
COPY --from=build /app/server .

# tell we are exposing our service on ports 8080 8081
EXPOSE 8080 8081

# run it!
CMD ["./server"]
