package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	gmail "google.golang.org/api/gmail/v1"
)

// BackupGMail is an example that demonstrates calling the Gmail API.
// It iterates over all messages of a user that are larger
// than 5MB, sorts them by size, and then interactively asks the user to
// choose either to Delete, Skip, or Quit for each message.
//
// Example usage:
//   go build -o go-api-demo *.go
//   go-api-demo -clientid="my-clientid" -secret="my-secret" gmail
func BackupGMail(user User) {

	client := user.Config.Client(context.Background(), user.Token)

	svc, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to create Gmail service: %v", err)
	}

	DBC := MongoSession()
	mongoCM := DBC.DB("gmail").C("messages")
	mongoCA := DBC.DB("gmail").C("attachments")
	mongoCL := DBC.DB("gmail").C("labels")
	defer DBC.Close()

	pageToken := ""
	for {
		req := svc.Users.Threads.List(user.Email).Q("older_than:11y")
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			log.Fatalf("Unable to retrieve messages: %v", err)
		}

		log.Printf("Processing %v messages...\n", len(r.Threads))
		for _, thread := range r.Threads {

			threadSer := svc.Users.Threads.Get(user.Email, thread.Id)

			thread, err := threadSer.Do()
			if err != nil {
				log.Fatalf("Unable to retrieve treads: %v", err)
			}

			if len(thread.Messages) != 0 {

				var wgData sync.WaitGroup

				for _, msg := range thread.Messages {

					msgo := Message{
						MsgID:           msg.Id,
						Message:         msg,
						Owner:           user.Email,
						ThreadID:        msg.ThreadId,
						InternalDateRaw: msg.InternalDate,
						InternalDate:    time.Unix(msg.InternalDate, 0),
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
			}

		}

		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
}

//Message structure for messages
type Message struct {
	ID              bson.ObjectId  `json:"id" bson:"_id,omitempty"`
	MsgID           string         `json:"msgID" bson:"msgID,omitempty"`
	ThreadID        string         `json:"threadID" bson:"threadID,omitempty"`
	Owner           string         `json:"owner" bson:"owner,omitempty"`
	InternalDateRaw int64          `json:"internalDateRaw" bson:"internalDateRaw,omitempty"`
	InternalDate    time.Time      `json:"internalDate" bson:"internalDate,omitempty"`
	Message         *gmail.Message `json:"message" bson:"message,omitempty"`
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
