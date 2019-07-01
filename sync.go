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
	emailaddress "github.com/mcnijman/go-emailaddress"
	gmail "google.golang.org/api/gmail/v1"
)

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
	defer DBC.Close()

	// init save syncer
	CRUDSyncer(syncer, DBC)

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

	CRUDSyncer(syncer, DBC)
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

			SaveLabels(msg.LabelIds, user, DBC)

			CRUDRawMessage(msgo, DBC)
			CRUDThreadMessage(mtread, DBC)

		}

		CRUDThread(t, DBC)
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

					CRUDAttachment(a, DBC)

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
