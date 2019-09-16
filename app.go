package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	mgo "github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

var mgoSession *mgo.Session

// MongoSession generate mongo session
func MongoSession() *mgo.Session {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "loocpi_rates",
		Name:    "mongoSession",
	}

	if mgoSession == nil {
		var err error
		mgoSession, err = mgo.Dial(os.Getenv("MONGO_CONN"))
		if err != nil {
			HandleError(proc, "Failed to start the Mongo session", err, true)
		}

	}
	return mgoSession.Clone()
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
	Status        string    `json:"status" bson:"status,omitempty"`
	Msg           string    `json:"msg" bson:"msg,omitempty"`
	Loop          int       `json:"loop" bson:"loop,omitempty"`
	Count         int       `json:"count" bson:"count,omitempty"`
}

// SaveLog save log
func SaveLog(log ServiceLog) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "api",
		Name:    "saveLog",
	}

	if log.Status == "" {
		log.Status = "ok"
	}

	log.UniqueService = log.Type + "." + log.Service + "." + log.Name

	log.End = time.Now()
	log.Duration = log.End.Sub(log.Start).String()

	LogsDB := MongoSession()
	defer LogsDB.Close()

	TypeDBC := LogsDB.DB(os.Getenv("MONGO_DB")).C("_" + strings.ToLower(log.Type) + "Logs")
	err := TypeDBC.Insert(log)

	if err != nil {
		HandleError(proc, "insert TypeDBC ", err, true)
	}

	TypeStatusDBC := LogsDB.DB(os.Getenv("MONGO_DB")).C("_statusLogs")

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
