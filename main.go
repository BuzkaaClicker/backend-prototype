package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"database/sql"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	_ "github.com/uptrace/bun/driver/pgdriver"
)

const logPath = "log.txt"

func setupLogger(verbose bool) {
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: time.Stamp,
		FullTimestamp:   true,
	})
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logrus.Fatalf("Failed to open log file %s for output: %s", logPath, err)
	}

	logrus.SetOutput(io.MultiWriter(os.Stderr, logFile))
	logrus.RegisterExitHandler(func() {
		if logFile == nil {
			return
		}
		logFile.Close()
	})
}

func main() {
	flag.Parse()
	setupLogger(os.Getenv("verbose") == "true")
	logrus.Infoln("Starting.")

	pgDsn := os.Getenv("POSTGRES_DSN")
	if pgDsn == "" {
		logrus.Errorln("Environment variable POSTGRES_DSN is not set!")
		return
	}
	sqldb, err := sql.Open("pg", pgDsn)
	if err != nil {
		logrus.WithError(err).Errorln("Database open failed.")
		return
	}
	defer sqldb.Close()
	db := bun.NewDB(sqldb, pgdialect.New())

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "dzialam")
	})

	versionRouter := router.PathPrefix("/version").Subrouter()
	versionController := VersionController{Repo: &PgVersionRepo{DB: db}}
	versionRouter.HandleFunc("/latest", versionController.ServeList).Methods("GET")

	logrus.Infoln("Listening...")
	http.ListenAndServe("127.0.0.1:2137", router)
	logrus.Exit(0)
}
