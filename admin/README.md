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

MONGO_CONN="localhost:27017" MONGO_DB="gmail" URL="http://localhost:8080/" APP_PORT="8080" LOG="true" PRINT_ERROR="true" go run *.go

BUILDING CONTINER

- gitlab

    - build
    docker build -t registry.gitlab.com/tmcsolutions/gapp:v1 .

    - push
    docker push registry.gitlab.com/tmcsolutions/gapp:v1 

    - pull
    docker pull registry.gitlab.com/tmcsolutions/gapp:v1

- counter - pagging
- first email - last email dates
- search by from, to, snippet, subject, date

- labels
- default label
- serach labels

- email details

- attachment

- simplify sync

- login


