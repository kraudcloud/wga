FROM golang:alpine AS build

WORKDIR /app

COPY . .

RUN go build -o /go/bin/app 

FROM alpine:3

RUN apk --no-cache add wireguard-tools-wg-quick nftables

COPY --from=build /go/bin/app /bin/wga

ENTRYPOINT ["/bin/wga"]
