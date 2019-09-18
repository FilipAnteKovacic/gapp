package main

import (
	"time"

	"golang.org/x/oauth2"
	gmail "google.golang.org/api/gmail/v1"
	people "google.golang.org/api/people/v1"
)

// RefreshToken refresh token of user
func RefreshToken(user *User) {

	if user.Token.Expiry.Add(2*time.Hour).Format("2006-01-02T15:04:05") < time.Now().Format("2006-01-02T15:04:05") {

		tokenSource := user.Config.TokenSource(oauth2.NoContext, user.Token)
		sourceToken := oauth2.ReuseTokenSource(nil, tokenSource)
		newToken, _ := sourceToken.Token()

		if newToken.AccessToken != user.Token.AccessToken {
			user.Token = newToken
			UpdateUser(user.ID.Hex(), *user)
		}
	}

}

// GetGmailService refresh token, create client & return service
func GetGmailService(user User) *gmail.Service {

	proc := ServiceLog{
		Start:   time.Now(),
		Type:    "function",
		Service: "gapp",
		Name:    "GetGmailService",
	}

	// Refresh token
	RefreshToken(&user)

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

	// Refresh token
	RefreshToken(&user)

	// get client for using Gmail API
	client := user.Config.Client(oauth2.NoContext, user.Token)
	svc, err := people.New(client)
	if err != nil {
		HandleError(proc, "Unable to create Gmail service", err, true)
		return nil
	}

	return svc

}
