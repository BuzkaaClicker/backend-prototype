package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
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

	pgUrl, err := pg.ParseURL(os.Getenv("POSTGRES_URL"))
	if err != nil {
		logrus.WithError(err).Errorln("Invalid POSTGRES_URL environment variable.")
		return
	}
	db := pg.Connect(pgUrl)
	defer db.Close()

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "dzialam")
	})

	versionRouter := router.PathPrefix("/version").Subrouter()
	versionController := VersionController{Repo: &SqlVersionRepo{}}
	versionRouter.HandleFunc("/version/latest", versionController.ServeList).Methods("GET")

	http.ListenAndServe("127.0.0.1:2137", router)
	logrus.Exit(0)
}
