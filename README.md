# GApp
Golang + MongoDB - Backup data from gmail

* Emails + Attachments
* Labels
* Contacts 

Requires MongoDB database for storing data

### Usage
 - backup
 - apps integration, expl. CRM

* Register user
* Add JSON OAuth cliend ID
* Add syncers

### Install

```
clone project
```

* MONGO_CONN    - mongo connection string
* MONGO_DB      - mongo database name
* URL           - application url
* DEBUG         - print error in console

#### GO RUN
```
MONGO_CONN=localhost:27017 MONGO_DB=gmail URL=http://localhost:8080/ DEBUG=true go run *.go
```

#### DOCKER RUN
```
docker build -t gapp:v1 .
docker run  -e "MONGO_CONN=localhost:27017" -e "MONGO_DB=gmail" -e "URL=http://localhost:8080/" -e "DEBUG=true"  --name gapp gapp:v1
```

# Tasks

## Sync
- [ ] Handle exced limit

## Single email view
- [ ] Load message images
- [*] Threads collapse