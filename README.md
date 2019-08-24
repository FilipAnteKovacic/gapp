Register to backup

ENV
- MONGO_CONN    - string for mongo connection
- MONGO_DB      - app for gmail backup
- URL           - APP URL
- APP_PORT      - APP PORT

- PRINT_ERROR   - print error in console
- DEBUG         - print error in console
- LOG           - true/false

ENV USE
os.Getenv("MONGO_DB")

GO RUN PROGRAM
LOOP=60 go run *.go


MONGO_CONN="192.168.1.221:27017" MONGO_DB="gmail" URL="http://192.168.1.221:8400/" APP_PORT="80" LOG="true" PRINT_ERROR="true" go run *.go

MONGO_CONN="localhost:27017" MONGO_DB="gmail" URL="http://localhost:8080/" APP_PORT="8080" LOG="true" PRINT_ERROR="true" go run *.go

GIT
git init

GIT ADD REMOTE
git remote add origin https://gitlab.com/tmcsolutions/gapp.git

GIT ADD
git add .

GIT COMMIT
git commit -m "Initial commit"

GIT PUSH
git push -u origin master


BUILDING CONTINER

- gitlab

    - build
    docker build -t registry.gitlab.com/tmcsolutions/gapp:v1 .

    - push
    docker push registry.gitlab.com/tmcsolutions/gapp:v1 

    - pull
    docker pull registry.gitlab.com/tmcsolutions/gapp:v1


- init sync/daily
- delete mail after sync

- attachments view
- attachments search