FROM golang:1.24 as builder
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o buzzutils3 ./src/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /go/buzzutils3 .
USER nonroot:nonroot
VOLUME /app/runtime
CMD ["./buzzutils3"]