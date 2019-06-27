package main

import (
	"os"
	"time"

	"github.com/globalsign/mgo/bson"
)

// GetGMail return emails from db by user
func GetGMail(user User, treadID string) []Message {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "getGMails",
	}

	defer SaveLog(proc)

	var gdata []Message

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("messages")
	defer DB.Close()

	// group tredids

	err := DBC.Find(bson.M{"owner": user.Email, "threadid": treadID}).Sort("-internalDate").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return gdata
	}

	return gdata

}

// GetGMails return emails from db by user
func GetGMails(user User, label, search string, page int) (int, []Thread) {

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

// GetMessageBody goes trought message struct, find body, combine in string
func GetMessageBody(m Message) string {

	body := ""

	if m.Payload.Body.Data != "" {

		//sDec, _ := b64.StdEncoding.DecodeString(m.Payload.Body.Data)
		//body = body + string(sDec)

	}

	return body

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

	// group tredids

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

// GetGMailLabels return all labels from db by user
func GetGMailLabels(user User) []string {

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

	// group tredids

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
