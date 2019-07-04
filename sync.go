package main

import (
	"encoding/base64"
	"html/template"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/globalsign/mgo"
	emailaddress "github.com/mcnijman/go-emailaddress"
	"golang.org/x/oauth2"
	gmail "google.golang.org/api/gmail/v1"
)

// GetGService refresh token, create client & return service
func GetGService(user User) *gmail.Service {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "RefreshToken",
	}

	if user.Token.Expiry.Add(2*time.Hour).Format("2006-01-02T15:04:05") < time.Now().Format("2006-01-02T15:04:05") {

		tokenSource := user.Config.TokenSource(oauth2.NoContext, user.Token)
		sourceToken := oauth2.ReuseTokenSource(nil, tokenSource)
		newToken, _ := sourceToken.Token()

		if newToken.AccessToken != user.Token.AccessToken {
			user.Token = newToken
			UpdateUser(user.ID.Hex(), user)
		}
	}

	// get client for using Gmail API
	client := user.Config.Client(oauth2.NoContext, user.Token)
	svc, err := gmail.New(client)
	if err != nil {
		HandleError(proc, "Unable to create Gmail service", err, true)
		return nil
	}

	return svc

}

// SyncGLabels sync labels from gmail
func SyncGLabels(syncer Syncer) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "SyncGLabels",
	}

	defer SaveLog(proc)

	DBC := MongoSession()
	defer DBC.Close()

	user := GetUserByEmail(syncer.Owner)
	svc := GetGService(user)

	if svc != nil {

		syncer.Count = 0

		req := svc.Users.Labels.List(user.Email)

		r, err := req.Do()
		if err != nil {
			HandleError(proc, "Unable to retrieve threads", err, true)
			return
		}

		if len(r.Labels) != 0 {

			for _, label := range r.Labels {

				lreq := svc.Users.Labels.Get(user.Email, label.Id)

				lr, err := lreq.Do()
				if err != nil {
					HandleError(proc, "Unable to retrieve threads", err, true)
					return
				}

				l := Label{
					LabelID:               lr.Id,
					Owner:                 user.Email,
					Name:                  lr.Name,
					Type:                  lr.Type,
					LabelListVisibility:   lr.LabelListVisibility,
					MessageListVisibility: lr.MessageListVisibility,
					MessagesTotal:         lr.MessagesTotal,
					MessagesUnread:        lr.MessagesUnread,
					ThreadsTotal:          lr.ThreadsTotal,
					ThreadsUnread:         lr.ThreadsUnread,
				}

				if lr.Color != nil {
					if lr.Color.BackgroundColor != "" {
						l.BackgroundColor = lr.Color.BackgroundColor
					}

					if lr.Color.TextColor != "" {
						l.TextColor = lr.Color.TextColor
					}
				}

				syncer.Count++

				CRUDLabel(l, DBC)

			}

		}

		syncer.End = time.Now()

		CRUDSyncer(syncer, DBC)

	}

}

// SyncGMail use syncer struct to start sync from GMail api
func SyncGMail(syncer Syncer) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gmailSync",
		Name:    "SyncGMail",
	}

	defer SaveLog(proc)

	DBC := MongoSession()
	defer DBC.Close()

	user := GetUserByEmail(syncer.Owner)
	svc := GetGService(user)

	if svc != nil {

		syncer.Count = 0

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

				syncer.Count++

				wgCostDrv.Add(1)
				go ProccessGmailThread(user, thread, svc, DBC, &wgCostDrv)

			}

			wgCostDrv.Wait()

			pageToken = r.NextPageToken
			syncer.LastPageToken = pageToken

			CRUDSyncer(syncer, DBC)

			if r.NextPageToken == "" {
				break
			}

			svc = GetGService(user)

		}

		syncer.End = time.Now()

		CRUDSyncer(syncer, DBC)

	}
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
