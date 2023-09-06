FROM golang:latest AS compiling_stage
RUN mkdir -p /go/src/Skillfactory/pipeline
WORKDIR /go/src/Skillfactory/pipeline
ADD main.go .
ADD go.mod .
RUN go install .
 
FROM alpine:latest
LABEL version="1.0"
LABEL maintainer="Zhdan Baliuk<balyuk603@gmail.com>"
WORKDIR /root/
COPY --from=compiling_stage /go/bin/pipeline .
ENTRYPOINT ./pipeline