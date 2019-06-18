package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

//Page struct for pages
type Page struct {
	URL     string
	Logo    string
	Name    string
	View    string
	Contact template.HTML
}

//Route struct for routes
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

//Routes all routes slice
type Routes []Route

//Controller handle request&parse templates
func Controller(w http.ResponseWriter, r *http.Request) {

	uri := r.RequestURI

	var p Page

	fmt.Println(uri)

	switch uri {
	case "/":
		p = Page{Name: "Login", View: "login"}

		if r.Method == "POST" {

			fmt.Println(r.FormValue("email"))
			fmt.Println(r.FormValue("password"))

		}

		break
	case "/register":
		p = Page{Name: "Register", View: "register"}

		if r.Method == "POST" {

			fmt.Println(r.FormValue("email"))
			fmt.Println(r.FormValue("password"))
			fmt.Println(r.FormValue("credentials"))

		}

		break
	case "/auth":

		break
	}

	p.URL = os.Getenv("URL")

	parsedTemplate, err := template.ParseFiles(
		"template/index.html",
		"template/views/"+p.View+".html",
	)

	if err != nil {
		log.Println("Error ParseFiles:", err)
		return
	}

	err = parsedTemplate.Execute(w, p)

	if err != nil {
		log.Println("Error Execute:", err)
		return
	}

}

//RoutesList app routes list
var RoutesList = Routes{
	Route{"login", "GET", "/", Controller},
	Route{"login", "POST", "/", Controller},
	Route{"register", "GET", "/register", Controller},
	Route{"register", "POST", "/register", Controller},
	Route{"auth", "GET", "/auth", Controller},
	Route{"auth", "POST", "/auth", Controller},
}

//AddRoutes add all routes to app
func AddRoutes(router *mux.Router) *mux.Router {

	for _, route := range RoutesList {
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}

	return router

}

//StartApp start app on specify port
func StartApp() {

	muxRouter := mux.NewRouter().StrictSlash(true)
	router := AddRoutes(muxRouter)
	// add static file prefix
	router.PathPrefix("/").Handler(http.StripPrefix("/static", http.FileServer(http.Dir("static/"))))

	err := http.ListenAndServe(":"+os.Getenv("APP_PORT"), handlers.CompressHandler(router))
	if err != nil {
		log.Fatal("error starting http server :: ", err)
		return
	}

}
