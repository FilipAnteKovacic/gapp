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
	people "google.golang.org/api/people/v1"
)

// GetGmailService refresh token, create client & return service
func GetGmailService(user User) *gmail.Service {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetGmailService",
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

// GetPeopleService refresh token, create client & return service
func GetPeopleService(user User) *people.Service {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetPeopleService",
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
	svc, err := people.New(client)
	if err != nil {
		HandleError(proc, "Unable to create Gmail service", err, true)
		return nil
	}

	return svc

}

// SyncGPeople sync people from gmail
func SyncGPeople(syncer Syncer) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "SyncGPeople",
	}

	defer SaveLog(proc)

	DBC := MongoSession()

	defer DBC.Close()

	user := GetUserByEmail(syncer.Owner)
	svc := GetPeopleService(user)

	if svc != nil {

		syncer.Count = 0

		personFields := "emailAddresses,names,phoneNumbers,organizations"

		//Gmail API page loop
		pageToken := ""
		for {

			req := svc.People.Connections.List("people/me").PersonFields(personFields)
			if pageToken != "" {
				req.PageToken(pageToken)
			}
			r, err := req.Do()
			if err != nil {
				HandleError(proc, "Unable to retrieve threads", err, true)
				return
			}

			for _, person := range r.Connections {

				p := Contact{
					GID:   person.ResourceName,
					Owner: user.Email,
				}

				if len(person.Names) != 0 {
					p.FirstName = person.Names[0].GivenName
					p.LastName = person.Names[0].FamilyName
				}

				if len(person.Organizations) != 0 {
					p.Company = person.Organizations[0].Name
					p.Title = person.Organizations[0].Title
				}

				if len(person.EmailAddresses) != 0 {
					p.Email = person.EmailAddresses[0].Value
				}

				if len(person.PhoneNumbers) != 0 {
					p.Phone = person.PhoneNumbers[0].CanonicalForm
				}

				CRUDContact(p, DBC)

			}

			pageToken = r.NextPageToken
			syncer.LastPageToken = pageToken

			syncer.Count++

			CRUDSyncer(syncer, DBC)

			if r.NextPageToken == "" {
				break
			}

		}

	}

	syncer.End = time.Now()

	CRUDSyncer(syncer, DBC)

}

// SyncGLabels sync labels from gmail
func SyncGLabels(syncer Syncer) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "SyncGLabels",
	}

	defer SaveLog(proc)

	DBC := MongoSession()
	defer DBC.Close()

	user := GetUserByEmail(syncer.Owner)
	svc := GetGmailService(user)

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
		Service: "gapp",
		Name:    "SyncGMail",
	}

	defer SaveLog(proc)

	DBC := MongoSession()
	defer DBC.Close()

	user := GetUserByEmail(syncer.Owner)
	svc := GetGmailService(user)

	syncer.Status = "start"
	CRUDSyncer(syncer, DBC)

	if svc != nil {

		syncer.Count = 0

		var lastFirstMsgDate string

		//Gmail API page loop
		pageToken := ""

		if syncer.LastPageToken != "" {
			pageToken = syncer.LastPageToken
		}

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

			syncer.Status = "start page " + pageToken
			CRUDSyncer(syncer, DBC)

			var wgCostDrv sync.WaitGroup

			for _, thread := range r.Threads {

				syncer.Count++

				wgCostDrv.Add(1)
				go ProccessGmailThread(user, thread, syncer.DeleteEmail, svc, DBC, &lastFirstMsgDate, &wgCostDrv)

			}

			wgCostDrv.Wait()

			pageToken = r.NextPageToken
			syncer.NextPageToken = r.NextPageToken
			syncer.LastPageToken = pageToken
			syncer.Status = "end page " + pageToken

			if syncer.CreatedBy == "user" {
				syncer.LastFirstMsgDate = lastFirstMsgDate
			}

			CRUDSyncer(syncer, DBC)

			if r.NextPageToken == "" {
				break
			}

			svc = GetGmailService(user)

		}

		syncer.End = time.Now()
		syncer.Status = "end"

		CRUDSyncer(syncer, DBC)

	}

}

// ProccessGmailThread process single thread
func ProccessGmailThread(user User, thread *gmail.Thread, deleteMsgs string, svc *gmail.Service, DBC *mgo.Session, lastFirstMsgDate *string, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
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
	t.FirstMsgDate = ""
	t.LastMsgDate = ""

	if t.MsgCount != 0 {

		for key, msg := range thread.Messages {

			internalDate := time.Unix(msg.InternalDate/1000, 0)

			msgo := RawMessage{
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

			mtread := ThreadMessage{
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

			mtread = ProcessPayload(msg.Id, user, msg.Payload, svc, DBC, mtread)

			t.AttchCount = t.AttchCount + len(mtread.Attachments)

			if t.HistoryID == mtread.HistoryID {

				t.InternalDate = mtread.InternalDate
				t.Labels = mtread.Labels
				t.Subject = mtread.Subject
				t.From = mtread.From
				t.To = mtread.To
				t.CC = mtread.CC
				t.BCC = mtread.BCC
				t.Date = mtread.Date
				t.Time = mtread.Time
				t.Year = mtread.Year
				t.Month = mtread.Month
				t.Day = mtread.Day
				t.Hours = mtread.Hours
				t.Minutes = mtread.Minutes
				t.Seconds = mtread.Seconds

				if key == 0 {
					t.FirstMsgDate = mtread.Date

					if *lastFirstMsgDate == "" {
						*lastFirstMsgDate = mtread.Date
					}

					if *lastFirstMsgDate < mtread.Date {
						*lastFirstMsgDate = mtread.Date
					}

				}
				t.LastMsgDate = mtread.Date

			}

			CRUDRawMessage(msgo, DBC)
			CRUDThreadMessage(mtread, DBC)

			if deleteMsgs == "true" {
				if err := svc.Users.Messages.Delete(user.Email, msgo.MsgID).Do(); err != nil {
					HandleError(proc, "unable to delete message "+msgo.MsgID, err, true)
				}
			}

		}

		CRUDThread(t, DBC)
	}

	wgi.Done()
	return

}

// ProcessPayload proccess trough levels of message part
func ProcessPayload(msgID string, user User, p *gmail.MessagePart, svc *gmail.Service, DBC *mgo.Session, mtread ThreadMessage) ThreadMessage {

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
				MimeType: p.MimeType,
				Headers:  ah,
				Data:     attachment.Data,
			}

			if attachment.Size != 0 {
				a.Size = attachment.Size
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

		for _, p := range p.Parts {

			mtread = ProcessPayload(msgID, user, p, svc, DBC, mtread)

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

// DailySync generate system syncers by users
func DailySync() {

	for {

		DBC := MongoSession()
		defer DBC.Close()

		unfinishedSyncers := GetUnfinishedSyncers()
		if len(unfinishedSyncers) != 0 {

			for _, sync := range unfinishedSyncers {

				if sync.LastPageToken != "" {
					go SyncGMail(sync)
				}

			}

		}

		syncers := GetAllDailyUserGenSyncers()

		if len(syncers) != 0 {

			for _, sync := range syncers {

				initSyncID := sync.ID.Hex()

				lastSystemSync := GetLastSystemSync(initSyncID)

				if lastSystemSync.Owner != "" {
					sync = lastSystemSync
				}

				afterDate, _ := time.Parse("2006-01-02", sync.LastFirstMsgDate)

				query := "after:" + afterDate.Format("2006/01/02") + " before:" + afterDate.AddDate(0, 0, 1).Format("2006/01/02")

				s := Syncer{
					CreatedBy:        "system",
					Owner:            sync.Owner,
					Query:            query,
					Type:             initSyncID,
					DeleteEmail:      sync.DeleteEmail,
					Start:            time.Now(),
					LastFirstMsgDate: afterDate.AddDate(0, 0, 1).Format("2006-01-02"),
				}

				// init save syncer
				CRUDSyncer(s, DBC)

				go SyncGMail(s)

			}
		}

		time.Sleep(24 * time.Hour)

	}

}
