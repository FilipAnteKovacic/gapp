package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	gmail "google.golang.org/api/gmail/v1"
)

// gmailMain is an example that demonstrates calling the Gmail API.
// It iterates over all messages of a user that are larger
// than 5MB, sorts them by size, and then interactively asks the user to
// choose either to Delete, Skip, or Quit for each message.
//
// Example usage:
//   go build -o go-api-demo *.go
//   go-api-demo -clientid="my-clientid" -secret="my-secret" gmail
func gmailMain(client *http.Client, argv []string) {
	if len(argv) != 0 {
		fmt.Fprintln(os.Stderr, "Usage: gmail")
		return
	}

	svc, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to create Gmail service: %v", err)
	}

	DBC := MongoSession()
	mongoCM := DBC.DB("gmail").C("messages")
	mongoCA := DBC.DB("gmail").C("attachments")
	defer DBC.Close()

	pageToken := ""
	for {
		req := svc.Users.Threads.List(os.Getenv("ACCOUNT")).Q("older_than:10y")
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			log.Fatalf("Unable to retrieve messages: %v", err)
		}

		log.Printf("Processing %v messages...\n", len(r.Threads))
		for _, thread := range r.Threads {

			threadSer := svc.Users.Threads.Get(os.Getenv("ACCOUNT"), thread.Id)

			thread, err := threadSer.Do()
			if err != nil {
				log.Fatalf("Unable to retrieve messages: %v", err)
			}

			if len(thread.Messages) != 0 {

				var wgData sync.WaitGroup

				for _, msg := range thread.Messages {

					msgo := Message{
						Message:      msg,
						Owner:        os.Getenv("ACCOUNT"),
						ThreadID:     msg.ThreadId,
						InternalDate: msg.InternalDate,
						ID:           msg.Id,
					}

					wgData.Add(1)
					go CRUDMessages(msgo, mongoCM, &wgData)

					if len(msg.Payload.Parts) != 0 {

						for _, part := range msg.Payload.Parts {

							if part.Body.AttachmentId != "" {

								attachmentSer := svc.Users.Messages.Attachments.Get(os.Getenv("ACCOUNT"), msg.Id, part.Body.AttachmentId)

								attachment, err := attachmentSer.Do()
								if err != nil {
									log.Fatalf("Unable to retrieve attachment: %v", err)
								}

								attachment.AttachmentId = part.Body.AttachmentId

								CRUDAttachment(attachment, mongoCA)
							}

							// Decode msg body
							//sDec, _ := b64.StdEncoding.DecodeString(part.Body.Data)
							//fmt.Println(string(sDec))

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
	Owner    string
	ThreadID string
	ID       string
	*gmail.Message
	InternalDate int64
}

// CRUDMessages save messages
func CRUDMessages(msg Message, mongoC *mgo.Collection, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Count:   0,
		Type:    "proccess",
		Service: "loocpi_datagen",
		Name:    "CRUDAirport",
		Loop:    0,
	}

	queryCheck := bson.M{"owner": msg.Owner, "id": msg.Id, "threadid": msg.ThreadId}

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
		Count:   0,
		Type:    "proccess",
		Service: "loocpi_datagen",
		Name:    "CRUDAttachment",
		Loop:    0,
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
