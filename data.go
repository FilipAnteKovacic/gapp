package main

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"io"
	"log"
	"os"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	gmail "google.golang.org/api/gmail/v1"
)

// Syncer struct for sync queries
type Syncer struct {
	ID            bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Owner         string        `json:"owner" bson:"owner,omitempty"`
	Query         string        `json:"query" bson:"query,omitempty"`
	Start         time.Time     `json:"start" bson:"start,omitempty"`
	End           time.Time     `json:"end" bson:"end,omitempty"`
	Count         int           `json:"count" bson:"count,omitempty"`
	LastPageToken string        `json:"lastPageToken" bson:"lastPageToken,omitempty"`
}

// GetAllSyncers return all syncers by user
func GetAllSyncers(user User) []Syncer {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "GetGMailLabels",
	}

	defer SaveLog(proc)

	var gdata []Syncer

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("syncers")
	defer DB.Close()

	err := DBC.Find(bson.M{"owner": user.Email}).Sort("-start").All(&gdata)
	if err != nil {
		HandleError(proc, "get syncers", err, true)
		return gdata
	}

	return gdata

}

// CRUDSyncer save syncer
func CRUDSyncer(sync Syncer, DBC *mgo.Session) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDSyncer",
	}

	mongoC := DBC.DB(os.Getenv("MONGO_DB")).C("syncers")

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
func CRUDThread(thread Thread, DBC *mgo.Session) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDThread",
	}

	mongoC := DBC.DB(os.Getenv("MONGO_DB")).C("threads")

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
			"owner": user.Email,
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
func CRUDThreadMessage(msg ThreadMessage, DBC *mgo.Session) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDMessages",
	}

	mongoC := DBC.DB(os.Getenv("MONGO_DB")).C("messages")

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
func CRUDRawMessage(msg RawMessage, DBC *mgo.Session) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDMessages",
	}

	mongoC := DBC.DB(os.Getenv("MONGO_DB")).C("messagesRaw")

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

// Attachment struct for attachments
type Attachment struct {
	ID          bson.ObjectId     `json:"id" bson:"_id,omitempty"`
	GridID      bson.ObjectId     `json:"gridID" bson:"gridID,omitempty"`
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
func CRUDAttachment(attch Attachment, DBC *mgo.Session) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDAttachment",
	}

	mongoC := DBC.DB(os.Getenv("MONGO_DB")).C("attachments")

	queryCheck := bson.M{"owner": attch.Owner, "attachID": attch.AttachID}

	actRes := Attachment{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		if attch.Size > 150000 {

			DB := mgo.Database{
				Name:    os.Getenv("MONGO_DB"),
				Session: DBC,
			}
			gridFile, err := DB.GridFS("attachments").Create(attch.Filename)
			if err != nil {
				log.Fatalf("Unable to create gridfs: %v", err)
			}

			gridFile.SetContentType(attch.ContentType)
			gridFile.SetChunkSize(1024)

			attch.GridID = (gridFile.Id().(bson.ObjectId))

			decoded, err := base64.URLEncoding.DecodeString(attch.Data)
			if err != nil {
				log.Fatalf("Unable to decode attachment: %v", err)
			}
			reader := bytes.NewReader(decoded)

			// make a buffer to keep chunks that are read
			buf := make([]byte, 1024)
			for {
				// read a chunk
				n, err := reader.Read(buf)
				if err != nil && err != io.EOF {
					log.Fatalf("Could not read the input file: %v", err)
				}
				if n == 0 {
					break
				}
				// write a chunk
				if _, err := gridFile.Write(buf[:n]); err != nil {
					log.Fatalf("Could not write to GridFs for : %v"+gridFile.Name(), err)
				}
			}
			gridFile.Close()

			attch.Data = "gridFS"

		}

		err = mongoC.Insert(attch)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	/*
		change := bson.M{"$set": attch}
		err = mongoC.Update(queryCheck, change)
		if err != nil {
			HandleError(proc, "error while updateing row", err, true)
			return
		}
		return
	*/
}

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

// Label struct for mail labels
type Label struct {
	ID                    bson.ObjectId `json:"id" bson:"_id,omitempty"`
	LabelID               string        `json:"labelID" bson:"labelID"`
	Owner                 string        `json:"owner" bson:"owner,omitempty"`
	Name                  string        `json:"name" bson:"name,omitempty"`
	Type                  string        `json:"type" bson:"type,omitempty"`
	LabelListVisibility   string        `json:"labelListVisibility" bson:"labelListVisibility,omitempty"`
	MessageListVisibility string        `json:"messageListVisibility" bson:"messageListVisibility,omitempty"`
	MessagesTotal         int64         `json:"messagesTotal" bson:"messagesTotal,omitempty"`
	MessagesUnread        int64         `json:"messagesUnread" bson:"messagesUnread,omitempty"`
	ThreadsTotal          int64         `json:"threadsTotal" bson:"threadsTotal,omitempty"`
	ThreadsUnread         int64         `json:"threadsUnread" bson:"threadsUnread,omitempty"`
	BackgroundColor       string        `json:"backgroundColor" bson:"backgroundColor,omitempty"`
	TextColor             string        `json:"textColor" bson:"textColor,omitempty"`
}

// CRUDLabel save label
func CRUDLabel(label Label, DBC *mgo.Session) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "CRUDLabel",
	}

	mongoC := DBC.DB(os.Getenv("MONGO_DB")).C("labels")

	queryCheck := bson.M{"labelID": label.LabelID, "owner": label.Owner}

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

// GetLabels return all labels from db by user
func GetLabels(user User) []Label {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "GetGMailLabels",
	}

	defer SaveLog(proc)

	var gdata []Label

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("labels")
	defer DB.Close()

	err := DBC.Find(bson.M{"owner": user.Email}).Sort("-threadsTotal").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return gdata
	}

	return gdata

}

// GetLabelsList return all labels by type from db by user
func GetLabelsList(user User) (string, map[string]string) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "GetLabelsByType",
	}

	defer SaveLog(proc)

	firstLabel := ""
	var gdata []Label

	ls := make(map[string]string)

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("labels")
	defer DB.Close()

	err := DBC.Find(bson.M{"owner": user.Email}).Sort("-threadsTotal").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return firstLabel, ls
	}

	if len(gdata) != 0 {

		for k, l := range gdata {

			if k == 0 {
				firstLabel = l.Name
			}

			ls[l.LabelID] = l.Name

		}

	}

	return firstLabel, ls

}

// GetLabelsByType return all labels by type from db by user
func GetLabelsByType(user User) map[string][]Label {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "GetLabelsByType",
	}

	defer SaveLog(proc)

	var gdata []Label

	ls := make(map[string][]Label)

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("labels")
	defer DB.Close()

	err := DBC.Find(bson.M{"owner": user.Email}).Sort("-threadsTotal").All(&gdata)
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return ls
	}

	if len(gdata) != 0 {

		for _, l := range gdata {

			ls[l.Type] = append(ls[l.Type], l)

		}

	}

	return ls

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
