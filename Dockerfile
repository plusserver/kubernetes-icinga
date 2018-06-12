FROM golang:latest AS build
WORKDIR /go/src/github.com/Nexinto/kubernetes-icinga
COPY . .
RUN go get k8s.io/client-go/...
RUN go get -d ./...
RUN CGO_ENABLED=0 GOOS=linux go build -o kubernetes-icinga pkg/controller/*.go

FROM scratch
COPY --from=build /go/src/github.com/Nexinto/kubernetes-icinga /kubernetes-icinga
CMD ["/kubernetes-icinga"]
