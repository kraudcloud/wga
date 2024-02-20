FROM golang:alpine AS build

WORKDIR /app

COPY . .

RUN go build -o /go/bin/app 

FROM alpine:3

RUN apk --no-cache add wireguard-tools-wg-quick

WORKDIR /root/

COPY --from=build /go/bin/app .

CMD ["./app"]
