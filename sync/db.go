package main

import (
	"os"
	"time"

	mgo "github.com/globalsign/mgo"
)

var mgoSession *mgo.Session

// MongoSession generate mongo session
func MongoSession() *mgo.Session {

	proc := ServiceLog{
		Start:   time.Now(),
		Count:   0,
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

		mgoSession.SetSocketTimeout(1 * time.Hour)
		mgoSession.SetMode(mgo.Monotonic, true)

	}
	return mgoSession.Clone()
}
