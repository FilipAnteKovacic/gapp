package main

import (
	"context"
	"encoding/base64"
	"html/template"
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
	LastID       string        `json:"lastID" bson:"lastID,omitempty"`
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

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "BackupGMail",
	}

	defer SaveLog(proc)

	DBC := MongoSession()
	mongoCS := DBC.DB("gmail").C("syncers")
	defer DBC.Close()

	// init save syncer
	CRUDSyncer(syncer, mongoCS)

	user := GetUserByEmail(syncer.Owner)

	// get client for using Gmail API
	client := user.Config.Client(context.Background(), user.Token)

	svc, err := gmail.New(client)
	if err != nil {
		HandleError(proc, "Unable to create Gmail service", err, true)
		return
	}

	syncer.ThreadsCount = 0

	//Gmail API page loop
	pageToken := ""
	for {

		req := svc.Users.Threads.List(user.Email).Q(syncer.Query)
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			HandleError(proc, "Unable to retrieve threads", err, true)
			return
		}

		var wgCostDrv sync.WaitGroup

		log.Printf("Processing %v threads...\n", len(r.Threads))
		for _, thread := range r.Threads {

			syncer.ThreadsCount++
			syncer.LastID = string(thread.Id)

			wgCostDrv.Add(1)
			go ProccessGmailThread(user, thread, svc, DBC, &wgCostDrv)

		}

		wgCostDrv.Wait()

		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}

	syncer.End = time.Now()

	CRUDSyncer(syncer, mongoCS)
}

// ProccessGmailThread process single thread
func ProccessGmailThread(user User, thread *gmail.Thread, svc *gmail.Service, DBC *mgo.Session, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "ProccessGmailThread",
	}

	defer SaveLog(proc)

	mongoCT := DBC.DB("gmail").C("threads")
	mongoCM := DBC.DB("gmail").C("messages")
	mongoCMR := DBC.DB("gmail").C("messagesRaw")
	mongoCL := DBC.DB("gmail").C("labels")

	// Init thread
	t := Thread{
		Owner:     user.Email,
		ThreadID:  thread.Id,
		HistoryID: thread.HistoryId,
		Snippet:   thread.Snippet,
	}

	// Get thread details
	threadSer := svc.Users.Threads.Get(user.Email, thread.Id)

	thread, err := threadSer.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve tread: %v", err)
		wgi.Done()
		return
	}

	t.MsgCount = len(thread.Messages)
	t.AttchCount = 0

	if t.MsgCount != 0 {

		for _, msg := range thread.Messages {

			msgo := RawMessage{
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

			mtread := ThreadMessage{
				Owner:        user.Email,
				MsgID:        msg.Id,
				ThreadID:     msg.ThreadId,
				HistoryID:    msg.HistoryId,
				Headers:      ParseMessageHeaders(msg.Payload.Headers),
				Labels:       msg.LabelIds,
				Snippet:      msg.Snippet,
				InternalDate: time.Unix(msg.InternalDate/1000, 0),
			}

			if len(msg.Payload.Headers) != 0 {

				for _, h := range msg.Payload.Headers {

					switch h.Name {
					case "Subject":

						mtread.Subject = h.Value

						break
					case "From":

						mtread.From = h.Value
						emails := emailaddress.Find([]byte(h.Value), false)
						for _, e := range emails {
							mtread.FromEmails = append(mtread.FromEmails, strings.ToLower(e.String()))
						}

						break
					case "To":

						mtread.To = h.Value
						emails := emailaddress.Find([]byte(h.Value), false)
						for _, e := range emails {
							mtread.ToEmails = append(mtread.ToEmails, strings.ToLower(e.String()))
						}

						break
					case "Date":

						mtread.EmailDate = h.Value

						break
					}

				}

			}

			mtread = ProcessMessageParts(msg.Id, user, msg.Payload.Parts, svc, DBC, mtread)

			if t.HistoryID == mtread.HistoryID {

				t.InternalDate = mtread.InternalDate
				t.Labels = mtread.Labels
				t.Subject = mtread.Subject
				t.From = mtread.From
				t.FromEmails = mtread.FromEmails
				t.To = mtread.To
				t.ToEmails = mtread.ToEmails
				t.EmailDate = mtread.EmailDate

			}

			SaveLabels(msg.LabelIds, user, mongoCL)

			CRUDRawMessage(msgo, mongoCMR)
			CRUDThreadMessage(mtread, mongoCM)

		}

		CRUDThread(t, mongoCT)
	}

	wgi.Done()
	return

}

// ProcessMessageParts proccess trough levels of message part
func ProcessMessageParts(msgID string, user User, parts []*gmail.MessagePart, svc *gmail.Service, DBC *mgo.Session, mtread ThreadMessage) ThreadMessage {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "ProcessMessageParts",
	}

	defer SaveLog(proc)

	if len(parts) != 0 {

		mongoCA := DBC.DB("gmail").C("attachments")

		for _, p := range parts {

			//base64.StdEncoding.DecodeString(p.Body.Data)
			//base64.RawURLEncoding.DecodeString(p.Body.Data)
			switch p.MimeType {
			case "text/plain":

				mtread.TextRaw = p.Body.Data

				decoded, err := base64.URLEncoding.DecodeString(p.Body.Data)
				if err != nil {
					mtread.Text = err.Error()
				} else {
					mtread.Text = string(decoded)
				}

				break
			case "text/html":

				mtread.HTMLRaw = p.Body.Data

				decoded, err := base64.URLEncoding.DecodeString(p.Body.Data)
				if err != nil {
					mtread.HTML = template.HTML(err.Error())
				} else {
					mtread.HTML = template.HTML(string(decoded))
				}

				break

			default:

				if p.Body.AttachmentId != "" {

					attachmentSer := svc.Users.Messages.Attachments.Get(user.Email, msgID, p.Body.AttachmentId)

					attachment, err := attachmentSer.Do()
					if err != nil {
						HandleError(proc, "Unable to retrieve attachment", err, true)
					}

					ah := ParseMessageHeaders(p.Headers)

					a := Attachment{
						Owner:    user.Email,
						MsgID:    mtread.MsgID,
						ThreadID: mtread.ThreadID,
						AttachID: p.Body.AttachmentId,
						Filename: p.Filename,
						Size:     attachment.Size,
						MimeType: p.MimeType,
						Headers:  ah,
						Data:     attachment.Data,
					}

					am := MessageAttachment{
						Name:    p.Filename,
						AttacID: a.AttachID,
					}

					mtread.Attachments = append(mtread.Attachments, am)

					CRUDAttachment(a, mongoCA)

				}

				break
			}

			if len(p.Parts) != 0 {
				mtread = ProcessMessageParts(msgID, user, p.Parts, svc, DBC, mtread)
			}

		}

	}

	return mtread

}

// ParseMessageHeaders return header map
func ParseMessageHeaders(headers []*gmail.MessagePartHeader) map[string]string {

	h := make(map[string]string)

	if len(headers) != 0 {

		for _, hv := range headers {

			h[hv.Name] = hv.Value

		}

	}

	return h
}

// Thread struct for threads from email
type Thread struct {
	ID           bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Owner        string        `json:"owner" bson:"owner,omitempty"`
	ThreadID     string        `json:"threadID" bson:"threadID,omitempty"`
	HistoryID    uint64        `json:"historyID" bson:"historyID,omitempty"`
	From         string        `json:"from" bson:"from,omitempty"`
	FromEmails   []string      `json:"fromEmails" bson:"fromEmails,omitempty"`
	To           string        `json:"to" bson:"to,omitempty"`
	ToEmails     []string      `json:"toEmails" bson:"toEmails,omitempty"`
	EmailDate    string        `json:"emailDate" bson:"emailDate,omitempty"`
	Subject      string        `json:"subject" bson:"subject,omitempty"`
	Snippet      string        `json:"snippet" bson:"snippet,omitempty"`
	MsgCount     int           `json:"msgCount" bson:"msgCount,omitempty"`
	AttchCount   int           `json:"attchCount" bson:"attchCount,omitempty"`
	Labels       []string      `json:"labels" bson:"labels,omitempty"`
	InternalDate time.Time     `json:"internalDate" bson:"internalDate,omitempty"`
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

// ThreadMessage simplify msg struct from gmail
type ThreadMessage struct {
	ID           bson.ObjectId       `json:"id" bson:"_id,omitempty"`
	Owner        string              `json:"owner" bson:"owner,omitempty"`
	MsgID        string              `json:"msgID" bson:"msgID,omitempty"`
	HistoryID    uint64              `json:"historyID" bson:"historyID,omitempty"`
	ThreadID     string              `json:"threadID" bson:"threadID,omitempty"`
	Headers      map[string]string   `json:"headers" bson:"headers,omitempty"`
	From         string              `json:"from" bson:"from,omitempty"`
	FromEmails   []string            `json:"fromEmails" bson:"fromEmails,omitempty"`
	To           string              `json:"to" bson:"to,omitempty"`
	ToEmails     []string            `json:"toEmails" bson:"toEmails,omitempty"`
	EmailDate    string              `json:"emailDate" bson:"emailDate,omitempty"`
	Subject      string              `json:"subject" bson:"subject,omitempty"`
	Snippet      string              `json:"snippet" bson:"snippet,omitempty"`
	Labels       []string            `json:"labels" bson:"labels,omitempty"`
	Text         string              `json:"text" bson:"text,omitempty"`
	TextRaw      string              `json:"textRaw" bson:"textRaw,omitempty"`
	HTML         template.HTML       `json:"html" bson:"html,omitempty"`
	HTMLRaw      string              `json:"htmlRaw" bson:"htmlRaw,omitempty"`
	Attachments  []MessageAttachment `json:"attachments" bson:"attachments,omitempty"`
	InternalDate time.Time           `json:"internalDate" bson:"internalDate,omitempty"`
}

// MessageAttachment short attachment struct
type MessageAttachment struct {
	Name    string `json:"name" bson:"name,omitempty"`
	AttacID string `json:"attachID" bson:"attachID,omitempty"`
}

// CRUDThreadMessage save messages for view
func CRUDThreadMessage(msg ThreadMessage, mongoC *mgo.Collection) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDMessages",
	}

	queryCheck := bson.M{"owner": msg.Owner, "msgID": msg.MsgID, "threadID": msg.ThreadID}

	actRes := ThreadMessage{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(msg)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	change := bson.M{"$set": msg}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)

		return
	}
	return

}

//RawMessage structure for raw messages
type RawMessage struct {
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

// CRUDRawMessage save raw messages
func CRUDRawMessage(msg RawMessage, mongoC *mgo.Collection) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDMessages",
	}

	queryCheck := bson.M{"owner": msg.Owner, "msgID": msg.MsgID, "threadID": msg.ThreadID}

	actRes := RawMessage{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(msg)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	change := bson.M{"$set": msg}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)

		return
	}
	return

}

// Attachment struct for attachments
type Attachment struct {
	ID          bson.ObjectId     `json:"id" bson:"_id,omitempty"`
	Owner       string            `json:"owner" bson:"owner,omitempty"`
	AttachID    string            `json:"attachID" bson:"attachID,omitempty"`
	MsgID       string            `json:"msgID" bson:"msgID,omitempty"`
	ThreadID    string            `json:"threadID" bson:"threadID,omitempty"`
	Filename    string            `json:"filename" bson:"filename,omitempty"`
	Size        int64             `json:"size" bson:"size,omitempty"`
	MimeType    string            `json:"mimeType" bson:"mimeType,omitempty"`
	ContentType string            `json:"contentType" bson:"contentType,omitempty"`
	Headers     map[string]string `json:"headers" bson:"headers,omitempty"`
	Data        string            `json:"data" bson:"data,omitempty"`
}

// CRUDAttachment save attachment
func CRUDAttachment(attch Attachment, mongoC *mgo.Collection) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDAttachment",
	}

	queryCheck := bson.M{"owner": attch.Owner, "attachID": attch.AttachID}

	actRes := Attachment{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(attch)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	change := bson.M{"$set": attch}
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
