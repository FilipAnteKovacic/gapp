package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	mgo "github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/gorilla/securecookie"
)

var systemSession *mgo.Session
var syncSession *mgo.Session
var mgoSession *mgo.Session

// MongoSession generate mongo session
func MongoSession() *mgo.Session {

	if mgoSession == nil {
		var err error
		mgoSession, err = mgo.Dial(os.Getenv("MONGO_CONN"))
		if err != nil {
			log.Fatalln("Failed to start the Mongo session: ", err)
		}

	}
	return mgoSession.Clone()
}

// SystemMongoSession Generate active sessions for logs
func SystemMongoSession() *mgo.Session {

	var err error
	systemSession, err = mgo.Dial(os.Getenv("MONGO_CONN"))
	if err != nil {
		log.Fatalln("Failed to start the Mongo session: ", err)
	}
	return systemSession.Clone()
}

// SyncMongoSession gen  sessions for syncers
func SyncMongoSession() *mgo.Session {

	var err error
	syncSession, err = mgo.Dial(os.Getenv("MONGO_CONN"))
	if err != nil {
		log.Fatalln("Failed to start the Mongo session: ", err)
	}
	return syncSession.Clone()
}

//ServiceLog log structure
type ServiceLog struct {
	UniqueService string    `json:"uniqueService" bson:"uniqueService,omitempty"`
	Type          string    `json:"type" bson:"type,omitempty"`
	Service       string    `json:"service" bson:"service,omitempty"`
	Name          string    `json:"name" bson:"name,omitempty"`
	Start         time.Time `json:"start" bson:"start,omitempty"`
	End           time.Time `json:"end" bson:"end,omitempty"`
	Duration      string    `json:"duration" bson:"duration,omitempty"`
	Seconds       float64   `json:"seconds" bson:"seconds,omitempty"`
	Nanoseconds   int64     `json:"nanosec" bson:"nanosec,omitempty"`
	Status        string    `json:"status" bson:"status,omitempty"`
	Msg           string    `json:"msg" bson:"msg,omitempty"`
	Loop          int       `json:"loop" bson:"loop,omitempty"`
	Count         int       `json:"count" bson:"count,omitempty"`
}

// SaveLog save log
func SaveLog(log ServiceLog) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "SaveLog",
	}

	if log.Status == "" {
		log.Status = "ok"
	}

	log.UniqueService = log.Type + "." + log.Service + "." + log.Name

	log.End = time.Now()

	dur := log.End.Sub(log.Start)

	log.Duration = dur.String()
	log.Seconds = dur.Seconds()
	log.Nanoseconds = dur.Nanoseconds()

	TypeDBC := systemSession.DB(os.Getenv("MONGO_DB")).C("_" + strings.ToLower(log.Type) + "Logs")
	err := TypeDBC.Insert(log)

	if err != nil {
		HandleError(proc, "insert TypeDBC ", err, true)
	}

	TypeStatusDBC := systemSession.DB(os.Getenv("MONGO_DB")).C("_statusLogs")

	err = TypeStatusDBC.Update(bson.M{"uniqueService": log.UniqueService}, bson.M{"$set": log})
	if err != nil {
		HandleError(proc, "update TypeStatusDBC ", err, true)

		err = TypeStatusDBC.Insert(log)
		if err != nil {
			HandleError(proc, "insert TypeStatusDBC ", err, true)
			return
		}
		return
	}
	return

}

// HandleError handle error depends on ENV
func HandleError(proc ServiceLog, status string, err error, save bool) {

	if os.Getenv("DEBUG") == "true" {

		fmt.Println("------------")
		fmt.Println("----ERROR---")
		fmt.Println("------------")
		fmt.Println(proc)
		fmt.Println("------------")
		fmt.Println(status)
		fmt.Println("------------")
		fmt.Println(err)
		fmt.Println("------------")

	}

	if save {

		proc.Status = "error"
		proc.Msg = status + ":" + err.Error()
		go SaveLog(proc)
	}

	return
}

// N global notifications for app
var N = Notifications{
	HaveNotfications: false,
}

// AppRedirect redirect on url
func AppRedirect(w http.ResponseWriter, r *http.Request, route string, status int) {

	http.Redirect(w, r, os.Getenv("APP_URL")+route, status)
	return
}

// RemoveAllSessions remove users sessions
func RemoveAllSessions(w http.ResponseWriter) {

	ClearSession("session", w)

	return
}

//Cookie functions

// cookieHandler
var cookieHandler = securecookie.New(securecookie.GenerateRandomKey(64), securecookie.GenerateRandomKey(32))

// SetSession set session
func SetSession(id string, r http.ResponseWriter) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "SetSession",
	}

	defer SaveLog(proc)

	value := map[string]string{
		"uid": id,
	}

	encoded, err := cookieHandler.Encode("session", value)
	if err != nil {
		HandleError(proc, "cookie encode error:", err, true)
		return
	}

	cookie := &http.Cookie{
		Name:  "session",
		Value: encoded,
		Path:  "/",
	}

	http.SetCookie(r, cookie)
	return
}

// ClearSession clear session
func ClearSession(name string, r http.ResponseWriter) {

	cookie := &http.Cookie{
		Name:   name,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}

	http.SetCookie(r, cookie)
	return
}

// CookieValid check if valid cookie
func CookieValid(r *http.Request) string {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CookieValid",
	}

	defer SaveLog(proc)

	cookie, err := r.Cookie("session")

	if err != nil {
		HandleError(proc, "cookie read session error: ", err, true)
		return ""
	}

	cookieValue := make(map[string]string)
	err = cookieHandler.Decode("session", cookie.Value, &cookieValue)
	if err != nil {
		HandleError(proc, "cookie decode session error: ", err, true)
		return ""
	}

	return cookieValue["uid"]

}

// CheckAuth check if session exist
// c - if page is public set true
func CheckAuth(w http.ResponseWriter, r *http.Request, c bool, route string) bool {

	cookieExist := CookieValid(r)

	if c == true && cookieExist != "" && r.URL.String() != route {

		AppRedirect(w, r, route, 302)
		return true
	}

	if c == false && cookieExist == "" && r.URL.String() != route {

		AppRedirect(w, r, route, 302)
		return true
	}

	return false
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
