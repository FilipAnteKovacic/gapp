package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {

	StartApp()

}

//StartApp start app on specify port
func StartApp() {

	muxRouter := mux.NewRouter().StrictSlash(true)

	muxRouter.Handle("/login", AuthController).Methods("GET", "POST")
	muxRouter.Handle("/register", AuthController).Methods("GET", "POST")

	muxRouter.Handle("/token/{email}", TokenController).Methods("GET", "POST")

	muxRouter.Handle("/emails", MailsController).Methods("GET", "POST")
	muxRouter.Handle("/email/{treadID}", MailController).Methods("GET")

	// add static file prefix
	muxRouter.PathPrefix("/").Handler(http.StripPrefix("/static", http.FileServer(http.Dir("static/"))))

	err := http.ListenAndServe(":"+os.Getenv("APP_PORT"), handlers.CompressHandler(muxRouter))
	if err != nil {
		log.Fatal("error starting http server :: ", err)
		return
	}

}
