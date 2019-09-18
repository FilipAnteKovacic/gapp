package main

import (
	"os"
	"time"

	"github.com/globalsign/mgo/bson"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

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

// CreateUser create user
func CreateUser(user User) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "CreateUser",
	}

	defer SaveLog(proc)

	user.Created = time.Now()
	user.Modified = time.Now()

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("users")
	defer DB.Close()

	err := DBC.Insert(user)
	if err != nil {
		HandleError(proc, "insert new user", err, true)
	}

}

// UpdateUser update user
func UpdateUser(ID string, user User) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "UpdateUser",
	}

	defer SaveLog(proc)

	user.Modified = time.Now()

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("users")
	defer DB.Close()

	colQuerier := bson.M{"_id": bson.ObjectIdHex(ID)}
	change := bson.M{"$set": user}
	err := DBC.Update(colQuerier, change)
	if err != nil {
		HandleError(proc, "update user", err, true)
	}

}

// GetUser get single user from db
func GetUser(uid string) User {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetUser",
	}

	defer SaveLog(proc)

	row := User{}

	if uid != "" {

		DB := MongoSession()
		DBC := DB.DB(os.Getenv("MONGO_DB")).C("users")
		defer DB.Close()

		err := DBC.Find(bson.M{"_id": bson.ObjectIdHex(uid)}).Select(bson.M{"password": 0}).One(&row)
		if err != nil {
			HandleError(proc, "get user with id"+uid, err, true)
			return row
		}

	}

	return row

}

// GetUserByEmail get single user from db by email
func GetUserByEmail(email string) User {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetUserByEmail",
	}

	defer SaveLog(proc)

	row := User{}

	if email != "" {

		DB := MongoSession()
		DBC := DB.DB(os.Getenv("MONGO_DB")).C("users")
		defer DB.Close()

		err := DBC.Find(bson.M{"email": email}).Select(bson.M{"password": 0}).One(&row)
		if err != nil {
			HandleError(proc, "get user with id "+email, err, true)
			return row
		}

	}

	return row

}

// CheckUser check if user is active
func CheckUser(eml, pwd string) string {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "checkUser",
	}

	defer SaveLog(proc)

	DB := MongoSession()
	DBC := DB.DB(os.Getenv("MONGO_DB")).C("users")
	defer DB.Close()

	u := User{}
	err := DBC.Find(bson.M{"email": eml}).One(&u)
	if err != nil {
		HandleError(proc, "user not found: "+eml+" - error: ", err, true)
		return ""
	}

	if ComparePasswords(u.Password, pwd) {

		return u.ID.Hex()

	}

	return ""
}

// Hash

// HashAndSalt hash password
func HashAndSalt(pwd string) string {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "HashAndSalt",
	}

	defer SaveLog(proc)

	// Use GenerateFromPassword to hash & salt pwd.
	// MinCost is just an integer constant provided by the bcrypt
	// package along with DefaultCost & MaxCost.
	// The cost can be any value you want provided it isn't lower
	// than the MinCost (4)
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.MinCost)
	if err != nil {
		HandleError(proc, "error not encoded", err, true)
		return ""
	}
	// GenerateFromPassword returns a byte slice so we need to
	// convert the bytes to a string and return it
	return string(hash)
}

// ComparePasswords compare users password
func ComparePasswords(hashedPwd string, plainPwd string) bool {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "ComparePasswords",
	}

	defer SaveLog(proc)

	err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(plainPwd))
	if err != nil {
		HandleError(proc, "error not encoded", err, true)
		return false
	}

	return true
}
