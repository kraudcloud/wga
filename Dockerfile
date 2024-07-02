FROM golang:alpine AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o /go/bin/app 

FROM alpine:3

RUN apk --no-cache add wireguard-tools-wg-quick nftables unbound

COPY --from=build /go/bin/app /bin/wga

ENTRYPOINT ["/bin/wga"]
