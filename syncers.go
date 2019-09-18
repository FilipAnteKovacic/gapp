package main

import (
	"os"
	"time"

	"github.com/globalsign/mgo/bson"
)

// Syncer struct for sync queries
type Syncer struct {
	ID               bson.ObjectId `json:"id" bson:"_id,omitempty"`
	CreatedBy        string        `json:"createdBy" bson:"createdBy,omitempty"`
	Owner            string        `json:"owner" bson:"owner,omitempty"`
	Query            string        `json:"query" bson:"query,omitempty"`
	Type             string        `json:"type" bson:"type,omitempty"`
	DeleteEmail      string        `json:"deleteEmail" bson:"deleteEmail,omitempty"`
	Start            time.Time     `json:"start" bson:"start,omitempty"`
	End              time.Time     `json:"end" bson:"end,omitempty"`
	Duration         string        `json:"duration" bson:"duration,omitempty"`
	Count            int           `json:"count" bson:"count,omitempty"`
	LastPageToken    string        `json:"lastPageToken" bson:"lastPageToken,omitempty"`
	NextPageToken    string        `json:"nextPageToken" bson:"nextPageToken,omitempty"`
	LastFirstMsgDate string        `json:"lastFirstMsgDate" bson:"lastFirstMsgDate,omitempty"`
	Status           string        `json:"status" bson:"status,omitempty"`
}

// GetAllSyncers return all syncers by user
func GetAllSyncers(user User) []Syncer {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetAllSyncers",
	}

	defer SaveLog(proc)

	var gdata []Syncer
	DB := MongoSession()
	defer DB.Close()

	DBC := DB.DB(os.Getenv("MONGO_DB")).C("syncers")

	err := DBC.Find(bson.M{"owner": user.Email}).Sort("-start").All(&gdata)
	if err != nil {
		HandleError(proc, "get syncers", err, true)
		return gdata
	}

	return gdata

}

// GetUnfinishedSyncers return all syncers that are not completed
func GetUnfinishedSyncers() []Syncer {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetUnfinishedSyncers",
	}

	defer SaveLog(proc)

	var gdata []Syncer

	DB := MongoSession()
	defer DB.Close()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("syncers")

	err := DBC.Find(bson.M{"status": bson.M{"$ne": "end"}}).All(&gdata)
	if err != nil {
		HandleError(proc, "get syncers", err, true)
		return gdata
	}

	return gdata

}

// GetAllDailyUserGenSyncers return all syncers created by user, type: daily
func GetAllDailyUserGenSyncers() []Syncer {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetAllUserGenSyncers",
	}

	defer SaveLog(proc)

	DB := MongoSession()
	defer DB.Close()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("syncers")

	var gdata []Syncer

	err := DBC.Find(bson.M{"type": "daily", "createdBy": "user", "status": "end"}).All(&gdata)
	if err != nil {
		HandleError(proc, "get syncers", err, true)
		return gdata
	}

	return gdata

}

// GetLastSystemSync get system sync from id
func GetLastSystemSync(id string) Syncer {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetLastSystemSync",
	}

	defer SaveLog(proc)

	var s Syncer
	DB := MongoSession()
	defer DB.Close()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("syncers")

	err := DBC.Find(bson.M{"createdBy": "system", "type": id}).Sort("-start").One(&s)
	if err != nil {
		HandleError(proc, "get sync", err, true)
		return s
	}

	return s

}

// CRUDSyncer save syncer
func CRUDSyncer(sync Syncer) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CRUDSyncer",
	}

	defer SaveLog(proc)
	sync.Duration = sync.End.Sub(sync.Start).String()

	mongoC := syncSession.DB(os.Getenv("MONGO_DB")).C("syncers")

	queryCheck := bson.M{"owner": sync.Owner, "query": sync.Query, "start": sync.Start}

	actRes := Syncer{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(sync)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	change := bson.M{"$set": sync}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)
		return
	}
	return

}

// DailySync generate system syncers by users
func DailySync() {

	for {

		syncSession = SyncMongoSession()

		unfinishedSyncers := GetUnfinishedSyncers()

		if len(unfinishedSyncers) != 0 {

			for _, sync := range unfinishedSyncers {

				if sync.LastPageToken != "" {
					go SyncGMail(sync)
				}

			}

		}

		syncers := GetAllDailyUserGenSyncers()

		if len(syncers) != 0 {

			for _, sync := range syncers {

				initSyncID := sync.ID.Hex()

				lastSystemSync := GetLastSystemSync(initSyncID)

				if lastSystemSync.Owner != "" {
					sync = lastSystemSync
				}

				afterDate, _ := time.Parse("2006-01-02", sync.LastFirstMsgDate)

				query := "after:" + afterDate.Format("2006/01/02") + " before:" + afterDate.AddDate(0, 0, 1).Format("2006/01/02")

				s := Syncer{
					CreatedBy:        "system",
					Owner:            sync.Owner,
					Query:            query,
					Type:             initSyncID,
					DeleteEmail:      sync.DeleteEmail,
					Start:            time.Now(),
					LastFirstMsgDate: afterDate.AddDate(0, 0, 1).Format("2006-01-02"),
				}

				// init save syncer
				CRUDSyncer(s)

				go SyncGMail(s)

			}
		}

		time.Sleep(24 * time.Hour)

	}

}
