package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gorilla/securecookie"
	"golang.org/x/crypto/bcrypt"
)

// AppRedirect redirect on url
func AppRedirect(w http.ResponseWriter, r *http.Request, route string, status int) {

	http.Redirect(w, r, os.Getenv("APP_URL")+route, status)
	return
}

// RemoveAllSessions remove users sessions
func RemoveAllSessions(w http.ResponseWriter) {

	ClearSession("session", w)

	return
}

//Cookie functions

// cookieHandler
var cookieHandler = securecookie.New(securecookie.GenerateRandomKey(64), securecookie.GenerateRandomKey(32))

// SetSession set session
func SetSession(id string, r http.ResponseWriter) {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "setSession",
	}

	defer SaveLog(proc)

	value := map[string]string{
		"uid": id,
	}

	encoded, err := cookieHandler.Encode("session", value)
	if err != nil {
		HandleError(proc, "cookie encode error:", err, true)
		return
	}

	cookie := &http.Cookie{
		Name:  "session",
		Value: encoded,
		Path:  "/",
	}

	http.SetCookie(r, cookie)
	return
}

// ClearSession clear session
func ClearSession(name string, r http.ResponseWriter) {

	cookie := &http.Cookie{
		Name:   name,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}

	http.SetCookie(r, cookie)
	return
}

// CookieValid check if valid cookie
func CookieValid(r *http.Request) string {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "cookieValid",
	}

	defer SaveLog(proc)

	cookie, err := r.Cookie("session")

	if err != nil {
		HandleError(proc, "cookie read session error: ", err, true)
		return ""
	}

	cookieValue := make(map[string]string)
	err = cookieHandler.Decode("session", cookie.Value, &cookieValue)
	if err != nil {
		HandleError(proc, "cookie decode session error: ", err, true)
		return ""
	}

	return cookieValue["uid"]

}

// CheckAuth check if session exist
// c - if page is public set true
func CheckAuth(w http.ResponseWriter, r *http.Request, c bool, route string) bool {

	cookieExist := CookieValid(r)

	if c == true && cookieExist != "" && r.URL.String() != route {

		AppRedirect(w, r, route, 302)
		return true
	}

	if c == false && cookieExist == "" && r.URL.String() != route {

		AppRedirect(w, r, route, 302)
		return true
	}

	return false
}

// Hash

// HashAndSalt hash password
func HashAndSalt(pwd string) string {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "Function",
		Service: "admin",
		Name:    "hashAndSalt",
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
		Type:    "Function",
		Service: "admin",
		Name:    "comparePasswords",
	}

	defer SaveLog(proc)

	err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(plainPwd))
	if err != nil {
		HandleError(proc, "error not encoded", err, true)
		return false
	}

	return true
}
