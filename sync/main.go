package main

import (
	"os"
	"time"

	"github.com/globalsign/mgo/bson"
	"golang.org/x/oauth2"
)

func main() {

	Sync()

}

// Sync get users & start sync emails
func Sync() {

	users := GetUsers()

	if len(users) != 0 {

		for _, user := range users {

			BackupGMail(user)

		}

	}

}

// User struct in DB
type User struct {
	ID          bson.ObjectId  `json:"id" bson:"_id,omitempty"`
	Email       string         `json:"email" bson:"email,omitempty"`
	Password    string         `json:"password" bson:"password,omitempty"`
	Credentials []byte         `json:"credentials" bson:"credentials,omitempty"`
	Config      *oauth2.Config `json:"config" bson:"config,omitempty"`
	Token       *oauth2.Token  `json:"token" bson:"token,omitempty"`
	Created     time.Time      `json:"created" bson:"created,omitempty"`
	Modified    time.Time      `json:"modified" bson:"modified,omitempty"`
}

// GetUsers return array users from db
func GetUsers() []User {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "user",
		Name:    "GetUsers",
	}

	defer SaveLog(proc)

	var rows []User

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("users")
	defer DB.Close()

	err := DBC.Find(bson.M{}).Select(bson.M{"password": 0}).All(&rows)
	if err != nil {
		HandleError(proc, "get users", err, true)
		return rows
	}

	return rows

}
