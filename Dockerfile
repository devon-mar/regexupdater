FROM --platform=$BUILDPLATFORM golang:1.23rc1-alpine as builder
ARG TARGETOS TARGETARCH

WORKDIR /go/src/app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /bin/regexupdater .

FROM scratch
COPY --from=builder /etc/ssl/cert.pem /etc/ssl/cert.pem
COPY --from=builder /bin/regexupdater /bin/regexupdater
ENTRYPOINT ["/bin/regexupdater"]