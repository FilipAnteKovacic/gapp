package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// N global notifications for app
var N = Notifications{
	HaveNotfications: false,
}

func init() {

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

	err := http.ListenAndServe(":"+os.Getenv("APP_PORT"), handlers.CompressHandler(muxRouter))
	if err != nil {
		log.Fatal("error starting http server :: ", err)
		return
	}

}
