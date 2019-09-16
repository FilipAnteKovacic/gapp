FROM golang:1.12.1

WORKDIR /go/src/app/
COPY . /go/src/app/

RUN go get github.com/globalsign/mgo
RUN go get github.com/globalsign/mgo/bson
RUN go get github.com/gorilla/mux
RUN go get github.com/gorilla/handlers
RUN go get github.com/gorilla/securecookie
RUN go get golang.org/x/crypto/bcrypt
RUN go get github.com/mcnijman/go-emailaddress

RUN go get golang.org/x/oauth2
RUN go get golang.org/x/oauth2/google
RUN go get google.golang.org/api/gmail/v1

RUN go build
RUN go install

EXPOSE 8080

CMD ["app"]