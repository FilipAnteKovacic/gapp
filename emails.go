package main

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/globalsign/mgo/bson"
)

// ThreadMessage simplify msg struct from gmail
type ThreadMessage struct {
	ID        bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Owner     string        `json:"owner" bson:"owner,omitempty"`
	ThreadID  string        `json:"threadID" bson:"threadID,omitempty"`
	From      string        `json:"from" bson:"from,omitempty"`
	To        string        `json:"to" bson:"to,omitempty"`
	EmailDate string        `json:"emailDate" bson:"emailDate,omitempty"`
	Subject   string        `json:"subject" bson:"subject,omitempty"`
	Snippet   string        `json:"snippet" bson:"snippet,omitempty"`
	Labels    []string      `json:"labels" bson:"labels,omitempty"`
	Text      string        `json:"text" bson:"text,omitempty"`
	HTML      template.HTML `json:"html" bson:"html,omitempty"`
	//Attachments			string		`json:"html" bson:"html,omitempty"`
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

	var gdata []Message
	var tmsgs []ThreadMessage

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("messages")
	defer DB.Close()

	// group tredids

	err := DBC.Find(bson.M{"owner": user.Email, "threadID": treadID}).Sort("-internalDate").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return tmsgs
	}

	for _, msg := range gdata {

		tmsgs = append(tmsgs, SimplifyGMessage(msg))

	}

	return tmsgs

}

// SimplifyGMessage simplify message struct for view
func SimplifyGMessage(msg Message) ThreadMessage {

	var simply ThreadMessage

	simply.ThreadID = msg.ThreadID
	simply.Snippet = msg.Snippet
	simply.Labels = msg.Labels

	if len(msg.Payload.Headers) != 0 {

		for _, h := range msg.Payload.Headers {

			switch h.Name {
			case "Subject":

				simply.Subject = h.Value

				break
			case "From":

				simply.From = h.Value

				break
			case "To":

				simply.To = h.Value

				break
			case "Date":

				simply.EmailDate = h.Value

				break
			}

		}

	}

	if len(msg.Payload.Parts) != 0 {

		for _, p := range msg.Payload.Parts {

			switch p.MimeType {
			case "text/plain":

				decoded, err := base64.StdEncoding.DecodeString(p.Body.Data)
				if err != nil {
					fmt.Println("decode error text:", err)
				}

				simply.Text = string(decoded)

				break
			case "text/html":

				decoded, err := base64.RawURLEncoding.DecodeString(p.Body.Data)
				if err != nil {
					fmt.Println("decode error html:", err)
				}

				simply.HTML = template.HTML(string(decoded))

				break
			}

		}

	}

	return simply

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
