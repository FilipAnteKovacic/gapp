package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	emailaddress "github.com/mcnijman/go-emailaddress"
	gmail "google.golang.org/api/gmail/v1"
)

// Syncer struct for sync queries
type Syncer struct {
	ID           bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Owner        string        `json:"owner" bson:"owner,omitempty"`
	Query        string        `json:"query" bson:"query,omitempty"`
	Start        time.Time     `json:"start" bson:"start,omitempty"`
	End          time.Time     `json:"end" bson:"end,omitempty"`
	ThreadsCount int           `json:"count" bson:"count,omitempty"`
	LastID       int           `json:"lastID" bson:"lastID,omitempty"`
}

// CRUDSyncer save syncer
func CRUDSyncer(sync Syncer, mongoC *mgo.Collection) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDSyncer",
	}

	queryCheck := bson.M{"owner": sync.Owner, "query": sync.Query}

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

// BackupGMail use syncer struct to start sync from GMail api
func BackupGMail(syncer Syncer) {

	DBC := MongoSession()
	mongoCS := DBC.DB("gmail").C("syncers")
	mongoCT := DBC.DB("gmail").C("threads")
	mongoCM := DBC.DB("gmail").C("messages")
	mongoCA := DBC.DB("gmail").C("attachments")
	mongoCL := DBC.DB("gmail").C("labels")
	defer DBC.Close()

	CRUDSyncer(syncer, mongoCS)

	user := GetUserByEmail(syncer.Owner)

	client := user.Config.Client(context.Background(), user.Token)

	svc, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to create Gmail service: %v", err)
	}

	syncer.ThreadsCount = 0

	pageToken := ""
	for {

		req := svc.Users.Threads.List(user.Email).Q(syncer.Query)
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			log.Fatalf("Unable to retrieve messages: %v", err)
		}

		log.Printf("Processing %v messages...\n", len(r.Threads))
		for _, thread := range r.Threads {

			t := Thread{
				Owner:     user.Email,
				ThreadID:  thread.Id,
				HistoryID: thread.HistoryId,
				Snippet:   thread.Snippet,
			}

			threadSer := svc.Users.Threads.Get(user.Email, thread.Id)

			thread, err := threadSer.Do()
			if err != nil {
				log.Fatalf("Unable to retrieve treads: %v", err)
			}

			syncer.ThreadsCount++

			t.MsgCount = len(thread.Messages)

			if t.MsgCount != 0 {

				var wgData sync.WaitGroup

				for _, msg := range thread.Messages {

					msgo := Message{
						Owner:           user.Email,
						MsgID:           msg.Id,
						ThreadID:        msg.ThreadId,
						HistoryID:       msg.HistoryId,
						Labels:          msg.LabelIds,
						Snippet:         msg.Snippet,
						Payload:         msg.Payload,
						InternalDateRaw: msg.InternalDate,
						InternalDate:    time.Unix(msg.InternalDate/1000, 0),
					}

					if t.HistoryID == msgo.HistoryID {
						t.InternalDate = msgo.InternalDate
						t.InternalDateRaw = msgo.InternalDateRaw
						t.Labels = msgo.Labels

						if len(msg.Payload.Headers) != 0 {

							for _, h := range msg.Payload.Headers {

								switch h.Name {
								case "Subject":

									t.Subject = h.Value

									break
								case "From":

									t.From = h.Value

									emails := emailaddress.Find([]byte(h.Value), false)
									var emls []string
									for _, e := range emails {
										emls = append(emls, e.String())
									}
									t.FromEmail = strings.Join(emls, ",")

									break
								case "To":

									t.To = h.Value

									emails := emailaddress.Find([]byte(h.Value), false)
									for _, e := range emails {
										t.ToEmail = append(t.ToEmail, e.String())
									}

									break
								case "Date":

									t.EmailDate = h.Value

									break
								}

							}

						}

					}

					wgData.Add(1)
					go CRUDMessages(msgo, mongoCM, &wgData)

					SaveLabels(msg.LabelIds, user, mongoCL)

					if len(msg.Payload.Parts) != 0 {

						for _, part := range msg.Payload.Parts {

							if part.Body.AttachmentId != "" {

								attachmentSer := svc.Users.Messages.Attachments.Get(user.Email, msg.Id, part.Body.AttachmentId)

								attachment, err := attachmentSer.Do()
								if err != nil {
									log.Fatalf("Unable to retrieve attachment: %v", err)
								}

								attachment.AttachmentId = part.Body.AttachmentId

								CRUDAttachment(attachment, mongoCA)
							}

						}

					}

				}

				wgData.Wait()

				CRUDThread(t, mongoCT)
			}

		}

		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}

	syncer.End = time.Now()

	CRUDSyncer(syncer, mongoCS)
}

// Thread struct for threads from email
type Thread struct {
	ID              bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Owner           string        `json:"owner" bson:"owner,omitempty"`
	ThreadID        string        `json:"threadID" bson:"threadID,omitempty"`
	HistoryID       uint64        `json:"historyID" bson:"historyID,omitempty"`
	From            string        `json:"from" bson:"from,omitempty"`
	FromEmail       string        `json:"fromEmail" bson:"fromEmail,omitempty"`
	To              string        `json:"to" bson:"to,omitempty"`
	ToEmail         []string      `json:"toEmail" bson:"toEmail,omitempty"`
	EmailDate       string        `json:"emailDate" bson:"emailDate,omitempty"`
	Subject         string        `json:"subject" bson:"subject,omitempty"`
	Snippet         string        `json:"snippet" bson:"snippet,omitempty"`
	MsgCount        int           `json:"msgCount" bson:"msgCount,omitempty"`
	Labels          []string      `json:"labels" bson:"labels,omitempty"`
	InternalDateRaw int64         `json:"internalDateRaw" bson:"internalDateRaw,omitempty"`
	InternalDate    time.Time     `json:"internalDate" bson:"internalDate,omitempty"`
}

// CRUDThread save attachment
func CRUDThread(thread Thread, mongoC *mgo.Collection) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDThread",
	}

	queryCheck := bson.M{"threadID": thread.ThreadID}

	actRes := Thread{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(thread)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	change := bson.M{"$set": thread}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)
		return
	}
	return

}

//Message structure for messages
type Message struct {
	ID              bson.ObjectId      `json:"id" bson:"_id,omitempty"`
	Owner           string             `json:"owner" bson:"owner,omitempty"`
	MsgID           string             `json:"msgID" bson:"msgID,omitempty"`
	ThreadID        string             `json:"threadID" bson:"threadID,omitempty"`
	HistoryID       uint64             `json:"historyID" bson:"historyID,omitempty"`
	Labels          []string           `json:"labels" bson:"labels,omitempty"`
	Snippet         string             `json:"snippet" bson:"snippet,omitempty"`
	Payload         *gmail.MessagePart `json:"payload" bson:"payload,omitempty"`
	InternalDateRaw int64              `json:"internalDateRaw" bson:"internalDateRaw,omitempty"`
	InternalDate    time.Time          `json:"internalDate" bson:"internalDate,omitempty"`
}

// CRUDMessages save messages
func CRUDMessages(msg Message, mongoC *mgo.Collection, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDMessages",
	}

	queryCheck := bson.M{"owner": msg.Owner, "msgID": msg.MsgID, "threadID": msg.ThreadID}

	actRes := Message{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(msg)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			wgi.Done()
			return
		}

		wgi.Done()
		return

	}

	change := bson.M{"$set": msg}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)
		wgi.Done()
		return
	}
	wgi.Done()
	return

}

// CRUDAttachment save attachment
func CRUDAttachment(att *gmail.MessagePartBody, mongoC *mgo.Collection) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDAttachment",
	}

	queryCheck := bson.M{"attachmentid": att.AttachmentId}

	actRes := gmail.MessagePartBody{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(att)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	change := bson.M{"$set": att}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)
		return
	}
	return

}

// Label struct for mail labels
type Label struct {
	ID    bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Owner string        `json:"owner" bson:"owner,omitempty"`
	Name  string        `json:"name" bson:"name,omitempty"`
}

// SaveLabels save labels
func SaveLabels(labels []string, user User, mongoC *mgo.Collection) {

	if len(labels) != 0 {

		for _, name := range labels {

			l := Label{
				Name:  name,
				Owner: user.Email,
			}

			CRUDLabel(l, mongoC)

		}

	}

}

// CRUDLabel save label
func CRUDLabel(label Label, mongoC *mgo.Collection) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDLabel",
	}

	queryCheck := bson.M{"name": label.Name, "owner": label.Owner}

	actRes := Label{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(label)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	change := bson.M{"$set": label}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)
		return
	}
	return

}
