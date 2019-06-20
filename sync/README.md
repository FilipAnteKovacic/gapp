Sync data

ENV
- MONGO_CONN    - string for mongo connection
- MONGO_DB      - app for gmail backup

- PRINT_ERROR   - print error in console
- DEBUG         - print error in console
- LOG           - true/false

ENV USE
os.Getenv("MONGO_DB")

GO RUN PROGRAM
LOOP=60 go run *.go

MONGO_CONN="localhost:27017" MONGO_DB="gmail" LOG="true" PRINT_ERROR="true" go run *.go

BUILDING CONTINER

- gitlab

    - build
    docker build -t registry.gitlab.com/tmcsolutions/gapp:v1 .

    - push
    docker push registry.gitlab.com/tmcsolutions/gapp:v1 

    - pull
    docker pull registry.gitlab.com/tmcsolutions/gapp:v1

