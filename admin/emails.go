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

// getGMails return emails from db by user
func getGMail(user User, treadID string) []Message {

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

// getGMails return emails from db by user
func getGMails(user User) []Snippet {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "getGMails",
	}

	defer SaveLog(proc)

	var snippets []Snippet
	var gdata []Message

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("messages")
	defer DB.Close()

	// group tredids

	err := DBC.Find(bson.M{"owner": user.Email, "message.labelids": "INBOX"}).Limit(10).Sort("-internaldate").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return snippets
	}

	if len(gdata) != 0 {

		for _, g := range gdata {

			s := ParseSnippet(g)

			snippets = append(snippets, s)

		}

	}

	return snippets

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

// Decode msg body
//sDec, _ := b64.StdEncoding.DecodeString(part.Body.Data)
//fmt.Println(string(sDec))
