package main

import (
	"os"
	"time"

	"github.com/globalsign/mgo/bson"
	people "google.golang.org/api/people/v1"
)

// Contact define simlify person struct from gmail
type Contact struct {
	ID        bson.ObjectId `json:"id" bson:"_id,omitempty"`
	GID       string        `json:"gid" bson:"gid,omitempty"`
	Owner     string        `json:"owner" bson:"owner,omitempty"`
	FirstName string        `json:"firstName" bson:"firstName,omitempty"`
	LastName  string        `json:"lastName" bson:"lastName,omitempty"`
	Company   string        `json:"company" bson:"company,omitempty"`
	Title     string        `json:"title" bson:"title,omitempty"`
	Email     string        `json:"email" bson:"email,omitempty"`
	Phone     string        `json:"phone" bson:"phone,omitempty"`
}

// GetAllContacts return all contacts by user
func GetAllContacts(user User) []Contact {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetAllContacts",
	}

	defer SaveLog(proc)

	var gdata []Contact

	DB := MongoSession()
	defer DB.Close()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("contacts")

	err := DBC.Find(bson.M{"owner": user.Email}).Sort("-start").All(&gdata)
	if err != nil {
		HandleError(proc, "get syncers", err, true)
		return gdata
	}

	return gdata

}

// CRUDContact save syncer
func CRUDContact(p Contact) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CRUDContact",
	}

	defer SaveLog(proc)

	mongoC := syncSession.DB(os.Getenv("MONGO_DB")).C("contacts")

	queryCheck := bson.M{"owner": p.Owner, "gid": p.GID}

	actRes := Syncer{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		err = mongoC.Insert(p)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			return
		}

		return

	}

	change := bson.M{"$set": p}
	err = mongoC.Update(queryCheck, change)
	if err != nil {
		HandleError(proc, "error while updateing row", err, true)
		return
	}
	return

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

	// Get user
	user := GetUserByEmail(syncer.Owner)

	// Get google service
	svc := GetPeopleService(user)

	syncer.Status = "start"
	if svc != nil {

		//Gmail people loop
		pageToken := ""
		for {

			conns, err := GetConnectionsList(svc, pageToken)
			if err != nil {
				HandleError(proc, "Unable to retrieve threads", err, true)
				break
			}
			// Proccess contacts
			contacts, len := ProcessConections(conns.Connections, user)

			// Add contacts count
			syncer.Count = syncer.Count + len

			// Save contacts to DB
			SaveContacts(contacts)

			// Check last token
			pageToken = conns.NextPageToken
			syncer.LastPageToken = pageToken

			// Save syncer
			CRUDSyncer(syncer)

			// Reset
			if conns.NextPageToken == "" {
				svc = nil
				conns = nil
				break
			}

		}

	}

	syncer.End = time.Now()
	syncer.Status = "end"
	CRUDSyncer(syncer)

	return

}

// GetConnectionsList get connections from api
func GetConnectionsList(svc *people.Service, pageToken string) (*people.ListConnectionsResponse, error) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetConnectionsList",
	}

	defer SaveLog(proc)

	personFields := "emailAddresses,names,phoneNumbers,organizations"

	// Request contacts from api
	req := svc.People.Connections.List("people/me").PersonFields(personFields)
	if pageToken != "" {
		req.PageToken(pageToken)
	}
	r, err := req.Do()

	return r, err

}

// ProcessConections standard people
func ProcessConections(people []*people.Person, user User) ([]Contact, int) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "ProcessConections",
	}

	defer SaveLog(proc)

	var contacts []Contact

	count := len(people)

	if count != 0 {
		for _, person := range people {

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

			contacts = append(contacts, p)

		}
	}
	return contacts, count
}

// SaveContacts standard people
func SaveContacts(contacts []Contact) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "SaveContacts",
	}

	defer SaveLog(proc)

	if len(contacts) != 0 {
		for _, c := range contacts {
			CRUDContact(c)
		}
	}

}
