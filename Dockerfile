FROM golang:latest AS build
WORKDIR /go/src/github.com/Soluto-Private/kubernetes-icinga
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o kubernetes-icinga pkg/controller/*.go

FROM scratch
COPY --from=build /go/src/github.com/Soluto-Private/kubernetes-icinga/kubernetes-icinga /kubernetes-icinga
CMD ["/kubernetes-icinga"]
