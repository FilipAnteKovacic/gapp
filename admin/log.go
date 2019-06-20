package main

import (
	"os"
	"strings"
	"time"

	"github.com/globalsign/mgo/bson"
)

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

	if os.Getenv("LOG") == "true" {

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

	return

}
