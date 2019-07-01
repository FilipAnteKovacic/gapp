package main

import (
	"os"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// GetAttachment return attachment
func GetAttachment(attachID string) Attachment {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "GetAttachment",
	}

	defer SaveLog(proc)

	var attach Attachment

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("attachments")
	defer DB.Close()

	// group tredids

	err := DBC.Find(bson.M{"attachID": attachID}).One(&attach)
	if err != nil {
		HandleError(proc, "get attachment", err, true)
		return attach
	}

	return attach

}

// GetAttachmentGridFS get attachment from GridFS
func GetAttachmentGridFS(attach Attachment) *mgo.GridFile {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "GetAttachmentGridFS",
	}

	DB := MongoSession()
	defer DB.Close()

	DBC := mgo.Database{
		Name:    os.Getenv("MONGO_DB"),
		Session: DB,
	}

	gridFile, err := DBC.GridFS("attachments").OpenId(attach.GridID)
	if err != nil {
		HandleError(proc, "get attachment", err, true)
		return nil
	}

	return gridFile

}

// GetThreadMessages return emails from db by user
func GetThreadMessages(user User, treadID string) []ThreadMessage {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "getGMails",
	}

	defer SaveLog(proc)

	var tmsgs []ThreadMessage

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("messages")
	defer DB.Close()

	// group tredids

	err := DBC.Find(bson.M{"owner": user.Email, "threadID": treadID}).Sort("-internalDate").All(&tmsgs)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return tmsgs
	}

	return tmsgs

}

// GetThread return thread by ID
func GetThread(threadID, owner string) Thread {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "GetThread",
	}

	defer SaveLog(proc)

	var thread Thread

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("threads")
	defer DB.Close()

	err := DBC.Find(bson.M{"owner": owner, "threadID": threadID}).One(&thread)
	if err != nil {
		HandleError(proc, "get thread", err, true)
		return thread
	}

	return thread

}

// GetThreads return emails from db by user
func GetThreads(user User, label, search string, page int) (int, []Thread) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "getGMails",
	}

	defer SaveLog(proc)

	var gcount int
	var threads []Thread

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("threads")
	defer DB.Close()

	// group tredids
	query := bson.M{"owner": user.Email, "labels": label}

	if search != "" {

		query = bson.M{"$or": []bson.M{
			bson.M{"snippet": bson.M{"$regex": search}},
			bson.M{"subject": bson.M{"$regex": search}},
			bson.M{"from": bson.M{"$regex": search}},
			bson.M{"to": bson.M{"$regex": search}},
		},
			"owner":  user.Email,
			"labels": label,
		}

	}

	gcount, err := DBC.Find(query).Sort("-internalDate").Count()
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return 0, threads
	}

	skip := page * 50

	err = DBC.Find(query).Skip(skip).Limit(50).Sort("-internalDate").All(&threads)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return 0, threads
	}

	return gcount, threads

}

// GStats email owner quick stats
type GStats struct {
	MinDate string
	MaxDate string
	Count   int
}

// GetGMailsStats return gmail stats
func GetGMailsStats(user User) GStats {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "getGMailsStats",
	}

	defer SaveLog(proc)

	var stats GStats

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("threads")
	defer DB.Close()

	msgCount, err := DBC.Find(bson.M{"owner": user.Email}).Count()
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return stats
	}

	stats.Count = msgCount

	if msgCount != 0 {

		var lastMsg Thread

		err = DBC.Find(bson.M{"owner": user.Email}).Limit(1).Sort("-internalDate").One(&lastMsg)
		if err != nil {
			HandleError(proc, "get snippets", err, true)
			return stats
		}

		stats.MaxDate = lastMsg.EmailDate

		var firstMsg Thread

		err = DBC.Find(bson.M{"owner": user.Email}).Limit(1).Sort("internalDate").One(&firstMsg)
		if err != nil {
			HandleError(proc, "get snippets", err, true)
			return stats
		}

		stats.MaxDate = firstMsg.EmailDate

	}

	return stats

}

// GetLabels return all labels from db by user
func GetLabels(user User) []string {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "GetGMailLabels",
	}

	defer SaveLog(proc)

	var gdata []Label
	var labels []string

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("labels")
	defer DB.Close()

	err := DBC.Find(bson.M{"owner": user.Email}).Sort("name").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return labels
	}

	if len(gdata) != 0 {

		for _, l := range gdata {
			labels = append(labels, l.Name)
		}

	}

	return labels

}
