package main

import (
	"encoding/base64"
	"html/template"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/globalsign/mgo/bson"
	emailaddress "github.com/mcnijman/go-emailaddress"
	gmail "google.golang.org/api/gmail/v1"
)

// Message simplify msg struct from gmail
type Message struct {
	ID           bson.ObjectId       `json:"id" bson:"_id,omitempty"`
	Owner        string              `json:"owner" bson:"owner,omitempty"`
	MsgID        string              `json:"msgID" bson:"msgID,omitempty"`
	HistoryID    uint64              `json:"historyID" bson:"historyID,omitempty"`
	ThreadID     string              `json:"threadID" bson:"threadID,omitempty"`
	Headers      map[string]string   `json:"headers" bson:"headers,omitempty"`
	Date         string              `json:"date" bson:"date,omitempty"`
	Year         string              `json:"year" bson:"year,omitempty"`
	Month        string              `json:"month" bson:"month,omitempty"`
	Day          string              `json:"day" bson:"day,omitempty"`
	Time         string              `json:"time" bson:"time,omitempty"`
	Hours        string              `json:"hours" bson:"hours,omitempty"`
	Minutes      string              `json:"minutes" bson:"minutes,omitempty"`
	Seconds      string              `json:"seconds" bson:"seconds,omitempty"`
	From         string              `json:"from" bson:"from,omitempty"`
	FromEmails   string              `json:"fromEmails" bson:"fromEmails,omitempty"`
	To           string              `json:"to" bson:"to,omitempty"`
	ToEmails     string              `json:"toEmails" bson:"toEmails,omitempty"`
	CC           string              `json:"cc" bson:"cc,omitempty"`
	CCEmails     string              `json:"ccEmails" bson:"ccEmails,omitempty"`
	BCC          string              `json:"bcc" bson:"bcc,omitempty"`
	BCCEmails    string              `json:"bccEmails" bson:"bccEmails,omitempty"`
	Subject      string              `json:"subject" bson:"subject,omitempty"`
	Snippet      string              `json:"snippet" bson:"snippet,omitempty"`
	Labels       []string            `json:"labels" bson:"labels,omitempty"`
	Text         string              `json:"text" bson:"text,omitempty"`
	HTML         template.HTML       `json:"html" bson:"html,omitempty"`
	Attachments  []MessageAttachment `json:"attachments" bson:"attachments,omitempty"`
	InternalDate time.Time           `json:"internalDate" bson:"internalDate,omitempty"`
}

// MessageAttachment short attachment struct
type MessageAttachment struct {
	MsgID    string            `json:"msgID" bson:"msgID,omitempty"`
	ThreadID string            `json:"threadID" bson:"threadID,omitempty"`
	AttacID  string            `json:"attachID" bson:"attachID,omitempty"`
	Filename string            `json:"filename" bson:"filename,omitempty"`
	MimeType string            `json:"mimeType" bson:"mimeType,omitempty"`
	Headers  map[string]string `json:"headers" bson:"headers,omitempty"`
}

// SaveMessages save messages
func SaveMessages(messages []Message) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "SaveMessages",
	}

	defer SaveLog(proc)

	if len(messages) != 0 {
		var wgMessages sync.WaitGroup
		for _, m := range messages {
			wgMessages.Add(1)
			go CRUDThreadMessage(m, &wgMessages)
		}
		wgMessages.Wait()
		return
	}
	return
}

// CRUDThreadMessage save messages for view
func CRUDThreadMessage(msg Message, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CRUDThreadMessage",
	}

	defer SaveLog(proc)

	MS := MongoSession()
	mongoC := MS.DB(os.Getenv("MONGO_DB")).C("messages")
	defer MS.Close()

	queryCheck := bson.M{"owner": msg.Owner, "msgID": msg.MsgID, "threadID": msg.ThreadID}

	actRes := Message{}
	err := mongoC.Find(queryCheck).Select(bson.M{"_id": 1}).One(&actRes)

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

// SaveRawMessages save raw messages
func SaveRawMessages(messages []RawMessage) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "SaveMessages",
	}

	defer SaveLog(proc)

	if len(messages) != 0 {
		var wgMessages sync.WaitGroup
		for _, m := range messages {
			wgMessages.Add(1)
			go CRUDRawMessage(m, &wgMessages)
		}
		wgMessages.Wait()
		return
	}
	return

}

// CRUDRawMessage save raw messages
func CRUDRawMessage(msg RawMessage, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CRUDRawMessage",
	}

	defer SaveLog(proc)

	MS := MongoSession()
	mongoC := MS.DB(os.Getenv("MONGO_DB")).C("messagesRaw")
	defer MS.Close()

	queryCheck := bson.M{"owner": msg.Owner, "msgID": msg.MsgID, "threadID": msg.ThreadID}

	actRes := RawMessage{}
	err := mongoC.Find(queryCheck).Select(bson.M{"_id": 1}).One(&actRes)

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

// GetThreadMessages return emails from db by user
func GetThreadMessages(user User, treadID string) []Message {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetThreadMessages",
	}

	defer SaveLog(proc)

	var tmsgs []Message

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

// RawMessageProccess set raw msg
func RawMessageProccess(msg *gmail.Message, user User) RawMessage {

	internalDate := time.Unix(msg.InternalDate/1000, 0)

	return RawMessage{
		Owner:           user.Email,
		MsgID:           msg.Id,
		ThreadID:        msg.ThreadId,
		HistoryID:       msg.HistoryId,
		Labels:          msg.LabelIds,
		Snippet:         msg.Snippet,
		Payload:         msg.Payload,
		InternalDateRaw: msg.InternalDate,
		InternalDate:    internalDate,
	}

}

// ProccessMessage set standard msh
func ProccessMessage(msg *gmail.Message, user User) Message {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "ProccessMessage",
	}

	defer SaveLog(proc)

	internalDate := time.Unix(msg.InternalDate/1000, 0)

	mtread := Message{
		Owner:        user.Email,
		MsgID:        msg.Id,
		ThreadID:     msg.ThreadId,
		HistoryID:    msg.HistoryId,
		Headers:      ParseMessageHeaders(msg.Payload.Headers),
		Labels:       msg.LabelIds,
		Snippet:      msg.Snippet,
		Date:         internalDate.Format("2006-01-02"),
		Time:         internalDate.Format("15:04:05"),
		Year:         internalDate.Format("2006"),
		Month:        internalDate.Format("01"),
		Day:          internalDate.Format("02"),
		Hours:        internalDate.Format("15"),
		Minutes:      internalDate.Format("04"),
		Seconds:      internalDate.Format("05"),
		InternalDate: internalDate,
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
				if len(emails) != 0 {
					var emls []string
					for _, e := range emails {
						emls = append(emls, strings.ToLower(e.String()))
					}
					mtread.FromEmails = strings.Join(emls, ",")
				}

				break
			case "To":

				mtread.To = h.Value
				emails := emailaddress.Find([]byte(h.Value), false)

				if len(emails) != 0 {
					var emls []string
					for _, e := range emails {
						emls = append(emls, strings.ToLower(e.String()))
					}
					mtread.ToEmails = strings.Join(emls, ",")
				}

				break

			case "Cc":

				mtread.CC = h.Value

				emails := emailaddress.Find([]byte(h.Value), false)
				if len(emails) != 0 {
					var emls []string
					for _, e := range emails {
						emls = append(emls, strings.ToLower(e.String()))
					}
					mtread.CCEmails = strings.Join(emls, ",")
				}

			case "Bcc":

				mtread.BCC = h.Value
				emails := emailaddress.Find([]byte(h.Value), false)

				if len(emails) != 0 {
					var emls []string
					for _, e := range emails {
						emls = append(emls, strings.ToLower(e.String()))
					}
					mtread.BCCEmails = strings.Join(emls, ",")
				}

				break

			}

		}

	}

	mtread = ProcessPayload(msg.Payload, mtread)

	return mtread
}

// ProccessMessages go tru msgs
func ProccessMessages(t gmail.Thread, user User) (Message, []Message, []RawMessage, []MessageAttachment) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "ProccessMessages",
	}

	defer SaveLog(proc)

	var addThread Message
	var messages []Message
	var messagesRaw []RawMessage
	var attachments []MessageAttachment

	if len(t.Messages) != 0 {

		for _, msg := range t.Messages {

			messagesRaw = append(messagesRaw, RawMessageProccess(msg, user))

			message := ProccessMessage(msg, user)

			if t.HistoryId == msg.HistoryId {
				addThread = message
			}

			messages = append(messages, message)

			if len(message.Attachments) != 0 {
				for _, a := range message.Attachments {
					attachments = append(attachments, a)
				}
			}

		}

	}

	return addThread, messages, messagesRaw, attachments

}

// ProcessPayload proccess trough levels of message part
func ProcessPayload(p *gmail.MessagePart, mtread Message) Message {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "ProcessPayload",
	}

	defer SaveLog(proc)

	//base64.StdEncoding.DecodeString(p.Body.Data)
	//base64.RawURLEncoding.DecodeString(p.Body.Data)
	switch p.MimeType {
	case "text/plain":

		decoded, err := base64.URLEncoding.DecodeString(p.Body.Data)
		if err != nil {
			mtread.Text = mtread.Text + err.Error()
		} else {
			mtread.Text = mtread.Text + string(decoded)
		}

		break
	case "text/html":

		decoded, err := base64.URLEncoding.DecodeString(p.Body.Data)
		if err != nil {
			mtread.HTML = mtread.HTML + template.HTML(err.Error())
		} else {
			mtread.HTML = mtread.HTML + template.HTML(string(decoded))
		}

		break

	default:

		if p.Body.AttachmentId != "" {

			am := MessageAttachment{
				ThreadID: mtread.ThreadID,
				MsgID:    mtread.MsgID,
				AttacID:  p.Body.AttachmentId,
				Filename: p.Filename,
				MimeType: p.MimeType,
				Headers:  ParseMessageHeaders(p.Headers),
			}

			mtread.Attachments = append(mtread.Attachments, am)

		}

		break
	}

	if len(p.Parts) != 0 {

		for _, p := range p.Parts {

			mtread = ProcessPayload(p, mtread)

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

// DeleteMessages delete emails from gmail
func DeleteMessages(svc *gmail.Service, user User, msgs []Message) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "DeleteMessages",
	}

	defer SaveLog(proc)

	if len(msgs) != 0 {

		for _, msg := range msgs {
			if err := svc.Users.Messages.Delete(user.Email, msg.MsgID).Do(); err != nil {
				HandleError(proc, "unable to delete message "+msg.MsgID, err, true)
			}
		}

	}

}
