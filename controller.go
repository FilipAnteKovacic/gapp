package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
	people "google.golang.org/api/people/v1"
)

//Page struct for pages
type Page struct {
	URL  string
	Logo string
	Name string
	View string
	N    Notifications
	User User
}

//SyncPage struct for email pages
type SyncPage struct {
	URL     string
	Logo    string
	Name    string
	View    string
	N       Notifications
	User    User
	Syncers []Syncer
}

//EsPage struct for email pages
type EsPage struct {
	URL       string
	Logo      string
	Name      string
	View      string
	N         Notifications
	User      User
	Stats     GStats
	Count     int
	Paggining GPagging
	Label     string
	LabelName string
	Search    string
	Labels    map[string][]Label
	Emails    []Thread
}

//EPage struct for email pages
type EPage struct {
	URL      string
	Logo     string
	Name     string
	View     string
	N        Notifications
	User     User
	Stats    GStats
	Labels   map[string][]Label
	Thread   Thread
	Messages []ThreadMessage
}

// GPagging stats
type GPagging struct {
	MinCount     int
	MaxCount     int
	NextPage     int
	PreviousPage int
}

// MailController handle other requests
var MailController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	redirect := CheckAuth(w, r, false, "/login")

	if !redirect {

		vars := mux.Vars(r)

		user := GetUser(CookieValid(r))
		stats := GetGMailsStats(user)
		labels := GetLabelsByType(user)

		thread := GetThread(vars["treadID"], user.Email)
		messages := GetThreadMessages(user, vars["treadID"])

		p := EPage{
			Name:     "Email",
			View:     "email",
			URL:      os.Getenv("URL"),
			User:     user,
			Stats:    stats,
			Labels:   labels,
			Thread:   thread,
			Messages: messages,
		}

		parsedTemplate, err := template.ParseFiles(
			"template/index.html",
			"template/header.html",
			"template/views/"+p.View+".html",
		)

		if err != nil {
			log.Println("Error ParseFiles: "+p.View, err)
			return
		}

		err = parsedTemplate.Execute(w, p)

		if err != nil {
			log.Println("Error Execute:", err)
			return
		}

	}

})

// AttachController get attachment & push to client on download
var AttachController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	redirect := CheckAuth(w, r, false, "/login")

	if !redirect {

		// URL vars
		vars := mux.Vars(r)

		attachID := vars["attachID"]

		a := GetAttachment(attachID)

		for key, val := range a.Headers {
			w.Header().Set(key, val)
		}

		w.Header().Set("Expires", "0")
		w.Header().Set("Content-Length", strconv.Itoa(int(a.Size)))

		if a.Data == "gridFS" {

			gridFile := GetAttachmentGridFS(a)

			defer gridFile.Close()

			fileHeader := make([]byte, 1024)
			gridFile.Read(fileHeader)

			gridFile.Seek(0, 0)
			io.Copy(w, gridFile)

			//http.ServeContent(w, r, attach.Filename, time.Now(), gridFile) // Use proper last mod time

		} else {

			decoded, err := base64.URLEncoding.DecodeString(a.Data)
			if err != nil {
				log.Fatalf("Unable to decode attachment: %v", err)
			}
			http.ServeContent(w, r, a.Filename, time.Now(), bytes.NewReader(decoded))

		}

	}

})

// MailsController handle other requests
var MailsController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	redirect := CheckAuth(w, r, false, "/login")

	if !redirect {

		user := GetUser(CookieValid(r))
		stats := GetGMailsStats(user)
		firstLabel, labelsList := GetLabelsList(user)
		labelsByType := GetLabelsByType(user)

		search := r.FormValue("search")

		label := ""
		label = r.FormValue("label")
		if label == "" {

			if firstLabel != "" && search == "" {
				label = firstLabel
			}

		}

		page := r.FormValue("page")

		if page == "" {
			page = "0"
		}

		pg, _ := strconv.Atoi(page)

		gp := GPagging{
			MinCount:     pg * 50,
			MaxCount:     (pg * 50) + 50,
			NextPage:     (pg + 1),
			PreviousPage: (pg - 1),
		}

		gcount, emails := GetThreads(user, label, search, pg)

		p := EsPage{
			Name:      "Emails",
			View:      "emails",
			URL:       os.Getenv("URL"),
			User:      user,
			Search:    search,
			Label:     label,
			Labels:    labelsByType,
			Emails:    emails,
			Count:     gcount,
			Paggining: gp,
			Stats:     stats,
		}

		if val, ok := labelsList[label]; ok {
			p.LabelName = val
		}

		parsedTemplate, err := template.ParseFiles(
			"template/index.html",
			"template/header.html",
			"template/views/"+p.View+".html",
		)

		if err != nil {
			log.Println("Error ParseFiles: "+p.View, err)
			return
		}

		err = parsedTemplate.Execute(w, p)

		if err != nil {
			log.Println("Error Execute:", err)
			return
		}

	}

})

// SyncController handle token requests
var SyncController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	redirect := CheckAuth(w, r, false, "/login")

	if !redirect {

		u := GetUser(CookieValid(r))

		if r.Method == "POST" {

			DBC := MongoSession()
			defer DBC.Close()

			if r.FormValue("labels") != "" && u.Token != nil {

				s := Syncer{
					Owner: u.Email,
					Query: "labels",
					Start: time.Now(),
				}

				// init save syncer
				CRUDSyncer(s, DBC)

				go SyncGLabels(s)

			}

			if r.FormValue("people") != "" && u.Token != nil {

				s := Syncer{
					Owner: u.Email,
					Query: "people",
					Start: time.Now(),
				}

				// init save syncer
				CRUDSyncer(s, DBC)

				go SyncGPeople(s)

			}

			if r.FormValue("start_sync") != "" && u.Token != nil {

				query := " "
				if r.FormValue("query") != "" {
					query = r.FormValue("query")
				}

				s := Syncer{
					Owner: u.Email,
					Query: query,
					Start: time.Now(),
				}

				// init save syncer
				CRUDSyncer(s, DBC)

				go SyncGMail(s)

			}
			/*
				if r.FormValue("password") != "" {

					// URL vars
					pass := r.FormValue("password")

					tok, err := u.Config.PasswordCredentialsToken(oauth2.NoContext, u.Email, pass)
					if err != nil {
						log.Fatalf("Unable to retrieve token from web: %v", err)
					}

					u.Token = tok

					UpdateUser(u.ID.Hex(), u)

				}
			*/
			if r.FormValue("cred") != "" {

				// Parse our multipart form, 10 << 20 specifies a maximum
				// upload of 10 MB files.
				r.ParseMultipartForm(10 << 20)
				// FormFile returns the first file for the given key `myFile`
				// it also returns the FileHeader so we can get the Filename,
				// the Header and the size of the file
				file, _, err := r.FormFile("credentials")
				if err != nil {
					fmt.Println("Error Retrieving the File")
					fmt.Println(err)
					return
				}
				defer file.Close()
				/*
					fmt.Printf("Uploaded File: %+v\n", handler.Filename)
					fmt.Printf("File Size: %+v\n", handler.Size)
					fmt.Printf("MIME Header: %+v\n", handler.Header)
				*/
				// read all of the contents of our uploaded file into a
				// byte array
				fileBytes, err := ioutil.ReadAll(file)
				if err != nil {
					fmt.Println(err)
				}

				//https://godoc.org/golang.org/x/oauth2/jwt#Config
				//JWTConfigFromJSON
				// need to try
				/*
					sourceToken, err := google.JWTAccessTokenSourceFromJSON(fileBytes, gmail.MailGoogleComScope)
					if err != nil {
						log.Fatal("jwt to conf | ", err)
					}
					st, _ := sourceToken.Token()

					u.Token = st

					UpdateUser(u.ID.Hex(), u)
				*/

				// If modifying these scopes, delete your previously saved token.json.
				config, err := google.ConfigFromJSON(fileBytes, gmail.MailGoogleComScope, people.ContactsScope)
				if err != nil {
					log.Fatalf("Unable to parse client secret file to config: %v", err)
				}

				config.RedirectURL = os.Getenv("URL") + "token/"

				u.Config = config
				u.Credentials = fileBytes

				UpdateUser(u.ID.Hex(), u)

				authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

				http.Redirect(w, r, authURL, 301)

			}

		}

		syncers := GetAllSyncers(u)

		p := SyncPage{
			Name:    "Sync",
			View:    "sync",
			URL:     os.Getenv("URL"),
			User:    u,
			Syncers: syncers,
		}

		parsedTemplate, err := template.ParseFiles(
			"template/index.html",
			"template/header.html",
			"template/views/"+p.View+".html",
		)

		if err != nil {
			log.Println("Error ParseFiles: "+p.View, err)
			return
		}

		err = parsedTemplate.Execute(w, p)

		if err != nil {
			log.Println("Error Execute:", err)
			return
		}

	}

})

// TokenController handle token requests
var TokenController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	redirect := CheckAuth(w, r, false, "/register")

	if !redirect {

		// URL vars
		code := r.FormValue("code")

		u := GetUser(CookieValid(r))
		tok, err := u.Config.Exchange(oauth2.NoContext, code)
		if err != nil {
			log.Fatalf("Unable to retrieve token from web: %v", err)
		}

		u.Token = tok

		UpdateUser(u.ID.Hex(), u)

		http.Redirect(w, r, os.Getenv("URL")+"/syncers/", 301)

	}

})

// AuthController handle other requests
var AuthController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	uri := r.RequestURI

	var p Page

	switch uri {
	case "/":

		AppRedirect(w, r, "/login", 302)
		return

		break
	case "/logout":

		p = Page{Name: "Logout", View: "logout"}

		RemoveAllSessions(w)
		AppRedirect(w, r, "/login", 302)

		break
	case "/login":
		p = Page{Name: "Login", View: "login"}

		redirect := CheckAuth(w, r, true, "/emails")

		if !redirect {

			if r.Method == "POST" {

				email := r.FormValue("email")
				password := r.FormValue("password")

				if email != "" && password != "" {

					uid := CheckUser(email, password)

					if uid != "" {

						SetSession(uid, w)

						AppRedirect(w, r, "/emails", 302)
						return
					}

					AddNotification("Login", "User not valid", "error", &N)
					AppRedirect(w, r, "/login", 302)
					return
				}

				AddNotification("Login", "Please fill required fields", "error", &N)
				AppRedirect(w, r, "/login", 302)
				return
			}

			if r.Method == "GET" {

				RemoveAllSessions(w)

				p.N = N

			}
		}

		break
	case "/register":

		redirect := CheckAuth(w, r, true, "/emails")

		if !redirect {

			p = Page{Name: "Register", View: "register"}

			if r.Method == "POST" {

				u := User{
					Email:    r.FormValue("email"),
					Password: HashAndSalt(r.FormValue("password")),
				}

				checkUser := GetUserByEmail(u.Email)
				if checkUser.ID.Hex() == "" {

					CreateUser(u)
					user := GetUserByEmail(u.Email)

					SetSession(user.ID.Hex(), w)

					AppRedirect(w, r, "/syncers", 302)

				} else {

					AddNotification("Login", "User already exist", "error", &N)
					AppRedirect(w, r, "/login", 302)

				}

			}

		}

		break

	}

	p.URL = os.Getenv("URL")

	parsedTemplate, err := template.ParseFiles(
		"template/index.html",
		"template/views/"+p.View+".html",
	)

	if err != nil {
		log.Println("Error ParseFiles: "+p.View, err)
		return
	}

	err = parsedTemplate.Execute(w, p)

	ClearNotification(&N)

	if err != nil {
		log.Println("Error Execute:", err)
		return
	}

})
