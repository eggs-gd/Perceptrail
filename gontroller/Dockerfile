FROM golang:alpine AS builder

WORKDIR /build

ADD go.mod .

COPY . .

RUN go build -o gontroller gontroller.go

FROM alpine

WORKDIR /build

COPY --from=builder /build/gontroller /build/gontroller

CMD [". /gontroller"]
