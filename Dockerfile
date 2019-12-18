FROM golang:1.13.5

RUN go install .

FROM alpine:3.10.3

