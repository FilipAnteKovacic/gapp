package main

import (
	"log"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func init() {

	// if we crash the go code, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	systemSession = SystemMongoSession()

	go DailySync()

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

	muxRouter.Handle("/contacts/", ContactsController).Methods("GET", "POST")
	muxRouter.Handle("/emails", MailsController).Methods("GET", "POST")
	muxRouter.Handle("/email/{treadID}", MailController).Methods("GET")
	muxRouter.Handle("/attachment/{attachID}", AttachController).Methods("GET")

	// add static file prefix
	muxRouter.PathPrefix("/").Handler(http.StripPrefix("/static", http.FileServer(http.Dir("static/"))))

	err := http.ListenAndServe(":8080", handlers.CompressHandler(muxRouter))
	if err != nil {
		log.Fatal("error starting http server :: ", err)
		return
	}

}
