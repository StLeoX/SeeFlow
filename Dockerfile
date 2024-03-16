FROM golang:1.21

WORKDIR .

COPY . .

RUN go mod tidy && go build

CMD ["./seeflow", "observe"]
