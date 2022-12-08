package main

import (
	"github.com/romangrechin/rss-viewer/aggregator"
	"github.com/romangrechin/rss-viewer/db"
	"github.com/romangrechin/rss-viewer/web"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type configStruct struct {
	DbUrl         string        `yaml:"db_conn"`
	Address       string        `yaml:"address"`
	Threads       int           `yaml:"threads"`
	SourceTimeout time.Duration `yaml:"source_timeout"`
}

func main() {
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal("CONFIG ERROR: ", err)
	}

	config := &configStruct{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		log.Fatal("CONFIG ERROR: ", err)
	}

	if config.DbUrl == "" {
		log.Fatal("CONFIG ERROR: database connection string cannot be empty")
	}

	if config.Address == "" {
		log.Fatal("CONFIG ERROR: http server address cannot be empty")
	}

	if config.SourceTimeout < time.Second {
		config.SourceTimeout = time.Second
		log.Println("CONFIG WARNING: feeds source timeout cannot be less than 1s (default :1s)")
	}

	if config.Threads < 1 {
		config.Threads = 1
		log.Println("CONFIG WARNING: threads cannot be less than 1 (default :1)")
	}

	err = db.Connect(config.DbUrl)
	if err != nil {
		log.Fatal("DB ERROR: ", err)
	}

	server, err := web.NewServer(config.Address, config.SourceTimeout)
	if err != nil {
		log.Fatal("HTTP ERROR: ", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	agg := aggregator.New(config.Threads, config.SourceTimeout)
	agg.Run()
	log.Println("aggregator started")
	server.Serve()
	log.Println("http server started")

	<-done

	server.Close()
	log.Println("http server stopped")
	agg.Stop()
	log.Println("aggregator stopped")
	db.Close()
}
