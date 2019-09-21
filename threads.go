package main

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/globalsign/mgo/bson"
	gmail "google.golang.org/api/gmail/v1"
)

// Thread struct for threads from email
type Thread struct {
	ID           bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Owner        string        `json:"owner" bson:"owner,omitempty"`
	ThreadID     string        `json:"threadID" bson:"threadID,omitempty"`
	HistoryID    uint64        `json:"historyID" bson:"historyID,omitempty"`
	Date         string        `json:"date" bson:"date,omitempty"`
	Year         string        `json:"year" bson:"year,omitempty"`
	Month        string        `json:"month" bson:"month,omitempty"`
	Day          string        `json:"day" bson:"day,omitempty"`
	Time         string        `json:"time" bson:"time,omitempty"`
	Hours        string        `json:"hours" bson:"hours,omitempty"`
	Minutes      string        `json:"minutes" bson:"minutes,omitempty"`
	Seconds      string        `json:"seconds" bson:"seconds,omitempty"`
	From         string        `json:"from" bson:"from,omitempty"`
	To           string        `json:"to" bson:"to,omitempty"`
	CC           string        `json:"cc" bson:"cc,omitempty"`
	BCC          string        `json:"bcc" bson:"bcc,omitempty"`
	BCCEmails    string        `json:"bccEmails" bson:"bccEmails,omitempty"`
	Subject      string        `json:"subject" bson:"subject,omitempty"`
	Snippet      string        `json:"snippet" bson:"snippet,omitempty"`
	MsgCount     int           `json:"msgCount" bson:"msgCount,omitempty"`
	FirstMsgDate string        `json:"firstMsgDate" bson:"firstMsgDate,omitempty"`
	LastMsgDate  string        `json:"lastMsgDate" bson:"lastMsgDate,omitempty"`
	AttchCount   int           `json:"attchCount" bson:"attchCount,omitempty"`
	Labels       []string      `json:"labels" bson:"labels,omitempty"`
	InternalDate time.Time     `json:"internalDate" bson:"internalDate,omitempty"`
}

// SaveThreads save threads
func SaveThreads(threads []Thread) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "SaveThreads",
	}

	defer SaveLog(proc)

	if len(threads) != 0 {
		var wgThread sync.WaitGroup
		for _, t := range threads {
			time.Sleep((1 * time.Second) / 10)
			wgThread.Add(1)
			go CRUDThread(t, &wgThread)
		}
		wgThread.Wait()
		return
	}
	return
}

// CRUDThread save attachment
func CRUDThread(thread Thread, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CRUDThread",
	}

	MS := MongoSession()
	mongoC := MS.DB(os.Getenv("MONGO_DB")).C("threads")
	defer MS.Close()

	queryCheck := bson.M{"threadID": thread.ThreadID, "owner": thread.Owner}

	actRes := Thread{}
	err := mongoC.Find(queryCheck).Select(bson.M{"_id": 1}).One(&actRes)

	if err != nil {

		err = mongoC.Insert(thread)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			wgi.Done()
			return
		}
		wgi.Done()
		return

	}

	change := bson.M{"$set": thread}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)
		wgi.Done()
		return
	}

	wgi.Done()
	return

}

//ESearch query builder for treads
type ESearch struct {
	Query   string `json:"query" bson:"query,omitempty"`
	From    string `json:"from" bson:"from,omitempty"`
	To      string `json:"to" bson:"to,omitempty"`
	Subject string `json:"subject" bson:"subject,omitempty"`
	Text    string `json:"text" bson:"text,omitempty"`
}

// GetThreads return emails from db by user
func GetThreads(user User, label string, page int, s ESearch) (int, []Thread) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetThreads",
	}

	defer SaveLog(proc)

	var gcount int
	var threads []Thread

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("threads")
	DBM := DB.DB(os.Getenv("MONGO_DB")).C("messages")
	defer DB.Close()

	// group tredids
	query := bson.M{"owner": user.Email, "labels": label}

	if s.Query != "" || (s.From != "" ||
		s.To != "" ||
		s.Subject != "" ||
		s.Text != "") {

		// Check msgs first & return threadIDs

		query = bson.M{}

		if s.Query != "" {

			query = bson.M{"$or": []bson.M{
				bson.M{"from": bson.M{"$regex": s.Query}},
				bson.M{"to": bson.M{"$regex": s.Query}},
				bson.M{"snippet": bson.M{"$regex": s.Query}},
				bson.M{"subject": bson.M{"$regex": s.Query}},
				bson.M{"text": bson.M{"$regex": s.Query}},
				bson.M{"html": bson.M{"$regex": s.Query}},
			},
				"owner": user.Email,
			}

		} else {

			subQuery := []bson.M{}

			if s.From != "" {
				subQuery = append(subQuery, bson.M{"from": bson.M{"$regex": s.From}})
			}
			if s.To != "" {
				subQuery = append(subQuery, bson.M{"to": bson.M{"$regex": s.To}})
			}

			if s.Subject != "" {
				subQuery = append(subQuery, bson.M{"subject": bson.M{"$regex": s.Subject}})
			}

			if s.Text != "" {
				subQuery = append(subQuery, bson.M{"snippet": bson.M{"$regex": s.Text}})
				subQuery = append(subQuery, bson.M{"text": bson.M{"$regex": s.Text}})
				subQuery = append(subQuery, bson.M{"html": bson.M{"$regex": s.Text}})
			}

			query = bson.M{"$or": subQuery, "owner": user.Email}

		}

		mcount, err := DBM.Find(query).Count()
		if err != nil {
			HandleError(proc, "get snippets", err, true)
			return 0, threads
		}

		if mcount != 0 {

			var tIDs []string
			var mthreads []Message

			err = DBC.Find(query).Select(bson.M{"threadID": 1}).All(&mthreads)
			if err != nil {
				HandleError(proc, "get snippets", err, true)
				return 0, threads
			}

			if len(mthreads) != 0 {

				for _, m := range mthreads {

					tIDs = append(tIDs, m.ThreadID)

				}

			}

			if len(tIDs) != 0 {

				query = bson.M{
					"threadID": bson.M{"$in": tIDs},
					"owner":    user.Email,
				}

				gcount, err := DBC.Find(query).Count()
				if err != nil {
					HandleError(proc, "get snippets", err, true)
					return 0, threads
				}

				if gcount != 0 {

					skip := page * 50

					err = DBC.Find(query).Skip(skip).Limit(50).Sort("-internalDate").All(&threads)
					if err != nil {
						HandleError(proc, "get snippets", err, true)
						return 0, threads
					}

					return gcount, threads

				}

				return gcount, threads
			}

		}

		return 0, threads

	}

	gcount, err := DBC.Find(query).Count()
	if err != nil {
		HandleError(proc, "get snippets", err, true)
		return 0, threads
	}

	if gcount != 0 {

		skip := page * 50

		err = DBC.Find(query).Skip(skip).Limit(50).Sort("-internalDate").All(&threads)
		if err != nil {
			HandleError(proc, "get snippets", err, true)
			return 0, threads
		}

	}

	return gcount, threads

}

// GetThread return thread by ID
func GetThread(threadID, owner string) Thread {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
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

// SyncGMail use syncer struct to start sync from GMail api
func SyncGMail(syncer Syncer) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "SyncGMail",
	}

	defer SaveLog(proc)

	// Get user
	user := GetUserByEmail(syncer.Owner)

	// Get google service
	svc := GetGmailService(user)

	// Save syncer start
	syncer.Status = "start"
	CRUDSyncer(syncer)

	if svc != nil {

		//Gmail API page loop
		pageToken := ""

		if syncer.LastPageToken != "" {
			pageToken = syncer.LastPageToken
		}

		for {

			threadsService, err := GetThreadListService(svc, user, syncer, pageToken)
			if err != nil {
				HandleError(proc, "get threads for syncer:"+syncer.ID.Hex(), err, true)
				syncer.Status = "error:" + err.Error()
				CRUDSyncer(syncer)
				break
			}

			syncer.Status = "start page " + pageToken
			CRUDSyncer(syncer)

			// Get threads details

			var threadsList []gmail.Thread

			var wgThread sync.WaitGroup
			for _, t := range threadsService.Threads {

				wgThread.Add(1)

				go GetThreadsDetails(&threadsList, t.Id, svc, user, &wgThread)

			}
			wgThread.Wait()

			// Proccess threads
			threads, messages, rawMessages, attachmentsList, firstDate, lastDate := ProccessThreads(threadsList, user)

			syncer.Count = syncer.Count + len(threads)

			// Save threads
			SaveThreads(threads)

			// Save messages
			SaveMessages(messages)

			// Save messages
			SaveRawMessages(rawMessages)

			// Proccess attachments
			attachments := ProccessAttachments(svc, user, attachmentsList)

			// Save attachemts
			SaveAttachments(attachments)

			// Delete threads
			if syncer.DeleteEmail == "true" {
				DeleteMessages(svc, user, messages)
			}

			// Check next token
			pageToken = threadsService.NextPageToken
			syncer.NextPageToken = threadsService.NextPageToken
			syncer.LastPageToken = pageToken
			syncer.Status = "end page " + pageToken

			// CHECKKECKEKCE
			if syncer.CreatedBy == "user" {

				if syncer.FirstMsgDate == "" {

					syncer.FirstMsgDate = firstDate

				}

				if syncer.FirstMsgDate < firstDate {
					syncer.FirstMsgDate = firstDate
				}

				if syncer.LastMsgDate == "" {
					syncer.LastMsgDate = lastDate
				}

				if syncer.LastMsgDate > lastDate {
					syncer.LastMsgDate = lastDate
				}

			}

			// Save syncer
			CRUDSyncer(syncer)

			threads = nil
			messages = nil
			rawMessages = nil
			attachments = nil
			attachmentsList = nil

			// Reset
			if threadsService.NextPageToken == "" {
				threadsService = nil
				svc = nil
				break
			}

			svc = GetGmailService(user)

		}
		// Save syncer
		syncer.End = time.Now()
		syncer.Status = "end"
		CRUDSyncer(syncer)
		return
	}
	return

}

// GetThreadListService get threads from api
func GetThreadListService(svc *gmail.Service, user User, syncer Syncer, pageToken string) (*gmail.ListThreadsResponse, error) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "GetThreadListService",
	}

	defer SaveLog(proc)

	req := svc.Users.Threads.List(user.Email).Q(syncer.Query)
	if pageToken != "" {
		req.PageToken(pageToken)
	}

	return req.Do()

}

// GetThreadsDetails get threads details from api
func GetThreadsDetails(threadsList *[]gmail.Thread, tID string, svc *gmail.Service, user User, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetThreadsDetails",
	}

	defer SaveLog(proc)

	// Get thread details
	thread, err := svc.Users.Threads.Get(user.Email, tID).Do()

	if err != nil {

		if strings.Contains("rateLimitExceeded", err.Error()) {

			time.Sleep(1 * time.Second)

			// Get thread details
			thread, err := svc.Users.Threads.Get(user.Email, tID).Do()

			if err != nil {

				HandleError(proc, "Unable to retrieve thread"+tID, err, true)
				wgi.Done()
				return

			}

			(*threadsList) = append((*threadsList), *thread)
			wgi.Done()
			return

		}

		HandleError(proc, "Unable to retrieve thread"+tID, err, true)
		wgi.Done()
		return
	}

	(*threadsList) = append((*threadsList), *thread)
	wgi.Done()
	return

}

//ProccessThreads standard threads
func ProccessThreads(threads []gmail.Thread, user User) ([]Thread, []Message, []RawMessage, []MessageAttachment, string, string) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "ProccessThreads",
	}

	defer SaveLog(proc)

	var lastMsg string
	var firstMsg string

	var threadsList []Thread
	var msgList []Message
	var rawMsgList []RawMessage
	var attachList []MessageAttachment

	if len(threads) != 0 {

		for _, thread := range threads {

			// Init thread
			t := Thread{
				Owner:     user.Email,
				ThreadID:  thread.Id,
				HistoryID: thread.HistoryId,
				Snippet:   thread.Snippet,
			}

			t.MsgCount = len(thread.Messages)
			t.AttchCount = 0
			t.FirstMsgDate = ""
			t.LastMsgDate = ""

			threadAdd, messages, rawMessages, attachments := ProccessMessages(thread, user)

			t.AttchCount = t.AttchCount + len(attachments)

			if t.HistoryID == threadAdd.HistoryID {

				t.InternalDate = threadAdd.InternalDate
				t.Labels = threadAdd.Labels
				t.Subject = threadAdd.Subject
				t.From = threadAdd.From
				t.To = threadAdd.To
				t.CC = threadAdd.CC
				t.BCC = threadAdd.BCC
				t.Date = threadAdd.Date
				t.Time = threadAdd.Time
				t.Year = threadAdd.Year
				t.Month = threadAdd.Month
				t.Day = threadAdd.Day
				t.Hours = threadAdd.Hours
				t.Minutes = threadAdd.Minutes
				t.Seconds = threadAdd.Seconds

			}

			if len(messages) != 0 {
				for key, m := range messages {
					msgList = append(msgList, m)

					if key == 0 {
						t.FirstMsgDate = m.Date
					}
					t.LastMsgDate = m.Date

				}
			}

			if len(rawMessages) != 0 {
				for _, rm := range rawMessages {
					rawMsgList = append(rawMsgList, rm)
				}
			}

			if len(attachments) != 0 {
				for _, a := range attachments {
					attachList = append(attachList, a)
				}
			}

			if firstMsg == "" {
				firstMsg = t.FirstMsgDate
			}

			if firstMsg < t.FirstMsgDate {
				firstMsg = t.FirstMsgDate
			}

			if lastMsg == "" {
				lastMsg = t.LastMsgDate
			}

			if lastMsg > t.LastMsgDate {
				lastMsg = t.LastMsgDate
			}

			threadsList = append(threadsList, t)

		}

	}

	return threadsList, msgList, rawMsgList, attachList, firstMsg, lastMsg
}
