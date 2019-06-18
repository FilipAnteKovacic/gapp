package main

import (
	"os"
	"time"

	"github.com/globalsign/mgo/bson"
)

//ServiceLog log structure
type ServiceLog struct {
	Start    time.Time
	End      time.Time
	Duration string
	Count    int
	Type     string
	Service  string
	Name     string
	Status   string
	Msg      string
	Loop     int
}

// SaveLog save log
func SaveLog(log ServiceLog) {

	if os.Getenv("LOG") == "true" {

		log.End = time.Now()
		log.Duration = log.End.Sub(log.Start).String()

		LogsDB := MongoSession()
		defer LogsDB.Close()

		TypeDBC := LogsDB.DB("logs").C(log.Type + "Logs")
		err := TypeDBC.Insert(log)

		if err != nil {
			HandleError(log, "insert TypeDBC ", err, false)
			return
		}

		TypeStatusDBC := LogsDB.DB("logs").C("logStatus")

		colQuerier := bson.M{"name": log.Name}
		change := bson.M{"$set": log}
		err = TypeStatusDBC.Update(colQuerier, change)
		if err != nil {
			HandleError(log, "update TypeStatusDBC ", err, false)

			err = TypeStatusDBC.Insert(log)
			if err != nil {
				HandleError(log, "insert TypeStatusDBC ", err, false)
				return
			}
			return
		}
		return
	}

	return
}
