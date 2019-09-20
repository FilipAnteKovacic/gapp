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

	"github.com/globalsign/mgo/bson"
	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
	people "google.golang.org/api/people/v1"
)

// Notifications contain all notif
type Notifications struct {
	HaveNotfications bool
	Notifications    []Notification
}

// Notification struct
type Notification struct {
	Title string
	Text  string
	Type  string
}

// AddNotification add notification to display on template
func AddNotification(Ntitle, Ntext, Ntype string, N *Notifications) {

	N.HaveNotfications = true
	N.Notifications = append(N.Notifications, Notification{
		Title: Ntitle,
		Text:  Ntext,
		Type:  Ntype,
	})
	return
}

// ClearNotification remove all
func ClearNotification(N *Notifications) {

	N.HaveNotfications = false
	N.Notifications = nil
	return
}

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

//ContactsPage struct for contacts list
type ContactsPage struct {
	URL      string
	Logo     string
	Name     string
	View     string
	N        Notifications
	User     User
	Contacts []Contact
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
	Search    ESearch
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
	Messages []Message
}

// GPagging stats
type GPagging struct {
	MinCount     int
	MaxCount     int
	NextPage     int
	PreviousPage int
}

// GStats email owner quick stats
type GStats struct {
	Labels      int
	Threads     int
	Messages    int
	Attachments int
}

// MailsController handle other requests
var MailsController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "controller",
		Service: "gapp",
		Name:    "MailsController",
	}

	defer SaveLog(proc)

	redirect := CheckAuth(w, r, false, "/login")

	if !redirect {

		user := GetUser(CookieValid(r))
		stats := GetGMailsStats(user)
		firstLabel, labelsList := GetLabelsList(user)
		labelsByType := GetLabelsByType(user)

		s := ESearch{
			Query:   r.FormValue("search[query]"),
			From:    r.FormValue("search[from]"),
			To:      r.FormValue("search[to]"),
			Subject: r.FormValue("search[subject]"),
			Text:    r.FormValue("search[text]"),
		}

		label := ""
		label = r.FormValue("label")
		if label == "" {

			if firstLabel != "" && s.Query == "" {
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

		gcount, emails := GetThreads(user, label, pg, s)

		p := EsPage{
			Name:      "Emails",
			View:      "emails",
			URL:       os.Getenv("URL"),
			User:      user,
			Search:    s,
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

// GetGMailsStats return gmail stats
func GetGMailsStats(user User) GStats {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetGMailsStats",
	}

	defer SaveLog(proc)

	var err error
	var stats GStats

	DB := MongoSession()
	defer DB.Close()

	DBCT := DB.DB(os.Getenv("MONGO_DB")).C("threads")

	stats.Threads, err = DBCT.Find(bson.M{"owner": user.Email}).Count()
	if err != nil {
		HandleError(proc, "get threads count", err, false)
		return stats
	}

	DBCM := DB.DB(os.Getenv("MONGO_DB")).C("messages")

	stats.Messages, err = DBCM.Find(bson.M{"owner": user.Email}).Count()
	if err != nil {
		HandleError(proc, "get messages counts", err, false)
		return stats
	}

	DBCA := DB.DB(os.Getenv("MONGO_DB")).C("attachments")

	stats.Attachments, err = DBCA.Find(bson.M{"owner": user.Email}).Count()
	if err != nil {
		HandleError(proc, "get attachments counts", err, false)
		return stats
	}

	DBCL := DB.DB(os.Getenv("MONGO_DB")).C("labels")

	stats.Labels, err = DBCL.Find(bson.M{"owner": user.Email}).Count()
	if err != nil {
		HandleError(proc, "get labels count", err, false)
		return stats
	}

	return stats

}

// MailController handle other requests
var MailController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "controller",
		Service: "gapp",
		Name:    "MailController",
	}

	defer SaveLog(proc)

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

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "controller",
		Service: "gapp",
		Name:    "AttachController",
	}

	defer SaveLog(proc)

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

			fileHeader := make([]byte, 261120)
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

// ContactsController handle token requests
var ContactsController = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "controller",
		Service: "gapp",
		Name:    "ContactsController",
	}

	defer SaveLog(proc)

	redirect := CheckAuth(w, r, false, "/login")

	if !redirect {

		u := GetUser(CookieValid(r))

		p := ContactsPage{
			Name:     "Contacts",
			View:     "contacts",
			URL:      os.Getenv("URL"),
			User:     u,
			Contacts: GetAllContacts(u),
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

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "controller",
		Service: "gapp",
		Name:    "SyncController",
	}

	defer SaveLog(proc)

	redirect := CheckAuth(w, r, false, "/login")

	if !redirect {

		u := GetUser(CookieValid(r))

		if r.Method == "POST" {

			syncSession = SyncMongoSession()

			if r.FormValue("labels") != "" && u.Token != nil {

				s := Syncer{
					CreatedBy: "user",
					Owner:     u.Email,
					Query:     "labels",
					Type:      "init",
					Start:     time.Now(),
				}

				// init save syncer
				CRUDSyncer(s)

				go SyncGLabels(s)

			}

			if r.FormValue("contacts") != "" && u.Token != nil {

				s := Syncer{
					CreatedBy: "user",
					Owner:     u.Email,
					Query:     "contacts",
					Type:      "init",
					Start:     time.Now(),
				}

				// init save syncer
				CRUDSyncer(s)

				go SyncGPeople(s)

			}

			if r.FormValue("gmail") != "" && u.Token != nil {

				query := " "
				if r.FormValue("query") != "" {
					query = r.FormValue("query")
				}

				s := Syncer{
					CreatedBy:   "user",
					Owner:       u.Email,
					Query:       query,
					Type:        r.FormValue("type"),
					DeleteEmail: r.FormValue("deleteEmail"),
					Start:       time.Now(),
				}

				// init save syncer
				CRUDSyncer(s)

				go SyncGMail(s)

			}

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
				//axt.labels.set
				/*
					sourceToken, err := google.JWTAccessTokenSourceFromJSON(fileBytes, gmail.MailGoogleComScope)
					if err != nil {
						log.Fatal("jwt to conf | ", err)
					}
					st, _ := sourceToken.Token()

					u.Token = st
					u.Credentials = fileBytes

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

	case "/logout":

		p = Page{Name: "Logout", View: "logout"}

		RemoveAllSessions(w)
		AppRedirect(w, r, "/login", 302)
		return

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
