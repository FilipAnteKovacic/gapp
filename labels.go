package main

import (
	"os"
	"time"

	"google.golang.org/api/gmail/v1"

	"github.com/globalsign/mgo/bson"
)

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
func CRUDLabel(label Label) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CRUDLabel",
	}

	defer SaveLog(proc)
	DB := MongoSession()
	defer DB.Close()
	mongoC := DB.DB(os.Getenv("MONGO_DB")).C("labels")

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
		Type:    "function",
		Service: "gapp",
		Name:    "GetLabels",
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
		Type:    "function",
		Service: "gapp",
		Name:    "GetLabelsList",
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
		Type:    "function",
		Service: "gapp",
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

// SyncGLabels sync labels from gmail
func SyncGLabels(syncer Syncer) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "SyncGLabels",
	}

	defer SaveLog(proc)

	// Get user
	user := GetUserByEmail(syncer.Owner)

	// Get google service
	svc := GetGmailService(user)

	syncer.Status = "start"
	if svc != nil {

		labls, err := GetServiceLabelsList(svc, user)
		if err != nil {
			HandleError(proc, "get labels for user"+user.Email, err, true)
			return
		}

		// Get labels details labels
		labelsDetails := GetLabelsDetails(svc, labls.Labels, user)

		// Proccess labels
		labels, len := ProccessLabels(labelsDetails, user)

		// Add labels count
		syncer.Count = syncer.Count + len

		// Save labels to DB
		SaveLabels(labels)

		// Reset
		labls = nil
		svc = nil

		// Save syncer
		syncer.End = time.Now()
		syncer.Status = "end"
		CRUDSyncer(syncer)
		return
	}

	return
}

// GetServiceLabelsList from gmail api
func GetServiceLabelsList(svc *gmail.Service, user User) (*gmail.ListLabelsResponse, error) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "proccess",
		Service: "gapp",
		Name:    "GetServiceLabelsList",
	}

	defer SaveLog(proc)

	req := svc.Users.Labels.List(user.Email)
	return req.Do()

}

// GetLabelsDetails labels details labels struct
func GetLabelsDetails(svc *gmail.Service, labeles []*gmail.Label, user User) []*gmail.Label {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetLabelsDetails",
	}

	defer SaveLog(proc)

	var lblDetails []*gmail.Label

	if len(labeles) != 0 {

		for _, label := range labeles {

			lreq := svc.Users.Labels.Get(user.Email, label.Id)

			lr, err := lreq.Do()
			if err != nil {
				HandleError(proc, "Unable to retrieve labelID "+label.Id, err, true)
				break
			}

			lblDetails = append(lblDetails, lr)

		}

	}

	return lblDetails
}

// ProccessLabels standard labels struct
func ProccessLabels(labeles []*gmail.Label, user User) ([]Label, int) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "ProccessLabels",
	}

	defer SaveLog(proc)

	var ll []Label

	count := len(labeles)

	if count != 0 {

		for _, lr := range labeles {

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

			ll = append(ll, l)

		}

	}

	return ll, count
}

// SaveLabels save labels
func SaveLabels(labels []Label) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "SaveLabels",
	}

	defer SaveLog(proc)

	if len(labels) != 0 {
		for _, l := range labels {
			CRUDLabel(l)
		}
	}

}
