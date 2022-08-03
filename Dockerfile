FROM golang:1.17
WORKDIR /app
COPY . .
RUN go build .

FROM debian:10.0-slim

RUN apt-get update
RUN apt-get install -y ca-certificates
RUN rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=0 /app/eng_bot .
EXPOSE 80
CMD ["./eng_bot"]