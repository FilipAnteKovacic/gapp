package main

import (
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// N global notifications for app
var N = Notifications{
	HaveNotfications: false,
}

func main() {

	StartApp()

}

//StartApp start app on specify port
func StartApp() {

	muxRouter := mux.NewRouter().StrictSlash(true)

	muxRouter.Handle("/", AuthController).Methods("GET", "POST")
	muxRouter.Handle("/login", AuthController).Methods("GET", "POST")
	muxRouter.Handle("/register", AuthController).Methods("GET", "POST")
	muxRouter.Handle("/logout", AuthController).Methods("GET", "POST")

	muxRouter.Handle("/token/", TokenController).Methods("GET", "POST")

	muxRouter.Handle("/syncers/", SyncController).Methods("GET", "POST")

	muxRouter.Handle("/emails", MailsController).Methods("GET", "POST")
	muxRouter.Handle("/email/{treadID}", MailController).Methods("GET")
	muxRouter.Handle("/attachment/{attachID}", AttachController).Methods("GET")

	// add static file prefix
	muxRouter.PathPrefix("/").Handler(http.StripPrefix("/static", http.FileServer(http.Dir("static/"))))

	err := http.ListenAndServe(":"+os.Getenv("APP_PORT"), handlers.CompressHandler(muxRouter))
	if err != nil {
		log.Fatal("error starting http server :: ", err)
		return
	}

}

// InArray check if exist in array
func InArray(val interface{}, array interface{}) (exists bool, index int) {
	exists = false
	index = -1

	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				index = i
				exists = true
				return
			}
		}
	}

	return
}

// RandStringBytes generate random string
func RandStringBytes(n int) string {

	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
