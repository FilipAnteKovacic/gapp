package main

import (
	"os"
	"time"

	"github.com/globalsign/mgo/bson"
	gmail "google.golang.org/api/gmail/v1"
)

// Snippet part of email
type Snippet struct {
	TreadID string
	Subject string
	From    string
	To      string
	Text    string
	Date    string
}

//Message structure for messages
type Message struct {
	Owner    string
	ThreadID string
	ID       string
	*gmail.Message
	InternalDate int64
}

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

	err := DBC.Find(bson.M{"owner": user.Email, "threadid": treadID}).Sort("-internaldate").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return gdata
	}

	/*
		if len(gdata) != 0 {

			for _, g := range gdata {

				s := ParseSnippet(g)

				snippets = append(snippets, s)

			}

		}
	*/
	return gdata

}

// GetGMails return emails from db by user
func GetGMails(user User, label, search string, page int) (int, []Snippet) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "getGMails",
	}

	defer SaveLog(proc)

	var gcount int
	var snippets []Snippet
	var gdata []Message

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("messages")
	defer DB.Close()

	// group tredids
	query := bson.M{"owner": user.Email, "message.labelids": label}

	if search != "" {

		query = bson.M{"$or": []bson.M{
			bson.M{"message.payload.headers.value": bson.M{"$regex": search}},
			bson.M{"message.payload.snippet": bson.M{"$regex": search}},
		},
			"owner":            user.Email,
			"message.labelids": label,
		}

	}

	gcount, err := DBC.Find(query).Sort("-internaldate").Count()
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return 0, snippets
	}

	skip := page * 50

	err = DBC.Find(query).Skip(skip).Limit(50).Sort("-internaldate").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return 0, snippets
	}

	if len(gdata) != 0 {

		for _, g := range gdata {

			s := ParseSnippet(g)

			snippets = append(snippets, s)

		}

	}

	return gcount, snippets

}

// ParseSnippet extract small amout of data from Message
func ParseSnippet(g Message) Snippet {

	var s Snippet

	s.TreadID = g.ThreadID
	s.Text = g.Message.Snippet

	if len(g.Payload.Headers) != 0 {

		for _, h := range g.Payload.Headers {

			switch h.Name {
			case "Subject":

				s.Subject = h.Value

				break
			case "From":

				s.From = h.Value

				break
			case "To":

				s.To = h.Value

				break
			case "Date":

				s.Date = h.Value

				break
			}

		}

	}

	return s
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
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("messages")
	defer DB.Close()

	// group tredids

	msgCount, err := DBC.Find(bson.M{"owner": user.Email}).Sort("-internaldate").Count()
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return stats
	}

	stats.Count = msgCount

	if msgCount != 0 {

		var lastMsg Message

		err = DBC.Find(bson.M{"owner": user.Email}).Limit(1).Sort("-internaldate").One(&lastMsg)
		if err != nil {
			HandleError(proc, "get snippets", err, true)
			return stats
		}

		lastSnp := ParseSnippet(lastMsg)
		if lastSnp.Date != "" {

			stats.MaxDate = lastSnp.Date

		}

		var firstMsg Message

		err = DBC.Find(bson.M{"owner": user.Email}).Limit(1).Sort("internaldate").One(&firstMsg)
		if err != nil {
			HandleError(proc, "get snippets", err, true)
			return stats
		}

		firstSnp := ParseSnippet(firstMsg)
		if firstSnp.Date != "" {

			stats.MinDate = firstSnp.Date

		}

	}

	return stats

}
