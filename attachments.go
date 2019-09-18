package main

import (
	"bytes"
	"encoding/base64"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	gmail "google.golang.org/api/gmail/v1"
)

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
func CRUDAttachment(attch Attachment, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CRUDAttachment",
	}

	defer SaveLog(proc)

	mongoC := syncSession.DB(os.Getenv("MONGO_DB")).C("attachments")

	queryCheck := bson.M{"owner": attch.Owner, "attachID": attch.AttachID}

	actRes := Attachment{}
	err := mongoC.Find(queryCheck).One(&actRes)

	if err != nil {

		if attch.Size > 150000 {

			DB := mgo.Database{
				Name:    os.Getenv("MONGO_DB"),
				Session: syncSession,
			}
			gridFile, err := DB.GridFS("attachments").Create(attch.Filename)
			if err != nil {
				log.Fatalf("Unable to create gridfs: %v", err)
				wgi.Done()
				return
			}

			gridFile.SetContentType(attch.ContentType)
			gridFile.SetChunkSize(1024)

			attch.GridID = (gridFile.Id().(bson.ObjectId))

			decoded, err := base64.URLEncoding.DecodeString(attch.Data)
			if err != nil {
				log.Fatalf("Unable to decode attachment: %v", err)
				wgi.Done()
				return
			}
			reader := bytes.NewReader(decoded)

			// make a buffer to keep chunks that are read
			buf := make([]byte, 1024)
			for {
				// read a chunk
				n, err := reader.Read(buf)
				if err != nil && err != io.EOF {
					log.Fatalf("Could not read the input file: %v", err)
					wgi.Done()
					return
				}
				if n == 0 {
					break
				}
				// write a chunk
				if _, err := gridFile.Write(buf[:n]); err != nil {
					log.Fatalf("Could not write to GridFs for : %v"+gridFile.Name(), err)
					wgi.Done()
					return
				}
			}
			gridFile.Close()

			attch.Data = "gridFS"

		}

		err = mongoC.Insert(attch)
		if err != nil {
			HandleError(proc, "error while inserting row", err, true)
			wgi.Done()
			return
		}
		wgi.Done()
		return

	}
	wgi.Done()
	return

}

// GetAttachmentsDetails from api
func GetAttachmentsDetails(attachments *[]Attachment, att MessageAttachment, user User, svc *gmail.Service, wgi *sync.WaitGroup) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "ProccessAGetAttachmentsDetailsttachments",
	}

	defer SaveLog(proc)

	a := Attachment{
		Owner:    user.Email,
		MsgID:    att.MsgID,
		ThreadID: att.ThreadID,
		AttachID: att.AttacID,
		Filename: att.Filename,
		MimeType: att.MimeType,
		Headers:  att.Headers,
	}

	attachmentSer := svc.Users.Messages.Attachments.Get(a.Owner, a.MsgID, a.AttachID)

	attachment, err := attachmentSer.Do()
	if err == nil {

		if attachment.Size != 0 {

			a.Size = attachment.Size

		}

		if attachment.Data != "" {
			a.Data = attachment.Data
		}

	} else {
		HandleError(proc, "Unable to retrieve attachment ID "+a.AttachID+"from msgID:"+a.MsgID, err, true)

	}

	(*attachments) = append((*attachments), a)
	wgi.Done()
}

// ProccessAttachments get attachment data
func ProccessAttachments(svc *gmail.Service, user User, attach []MessageAttachment) []Attachment {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "ProccessAttachments",
	}

	defer SaveLog(proc)

	var attachments []Attachment

	var wgAttach sync.WaitGroup

	if len(attach) != 0 {

		for _, att := range attach {

			wgAttach.Add(1)
			go GetAttachmentsDetails(&attachments, att, user, svc, &wgAttach)

		}

	}

	wgAttach.Wait()

	return attachments
}

// SaveAttachments save attachments
func SaveAttachments(attachments []Attachment) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "SaveAttachments",
	}

	defer SaveLog(proc)

	var wgAttach sync.WaitGroup
	if len(attachments) != 0 {
		for _, a := range attachments {
			wgAttach.Add(1)
			go CRUDAttachment(a, &wgAttach)
		}
	}

	wgAttach.Wait()
	return
}

// GetAttachment return attachment
func GetAttachment(attachID string) Attachment {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
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

// GetAttachmentGridFS get attachment from GridFS
func GetAttachmentGridFS(attach Attachment) *mgo.GridFile {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetAttachmentGridFS",
	}

	defer SaveLog(proc)

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
