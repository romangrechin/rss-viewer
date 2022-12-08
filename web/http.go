package web

import (
	"context"
	"errors"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"
)

type httpServer struct {
	srv           *http.Server
	sourceTimeout time.Duration
	wg            sync.WaitGroup
	tpl           *template.Template
}

func (hs *httpServer) Serve() {
	hs.wg.Add(1)
	go func() {
		defer func() {
			hs.wg.Done()
		}()
		if err := hs.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP ERROR: ", err)
		}
	}()

	return
}

func (hs *httpServer) Close() error {
	err := hs.srv.Shutdown(context.Background())
	hs.wg.Wait()
	return err
}

func NewServer(address string, sourceTimeout time.Duration) (*httpServer, error) {
	if address == "" {
		return nil, errors.New("HTTP ERROR: address can not be empty")
	}

	tpl, err := template.ParseFiles("./static/index.html")
	if err != nil {
		return nil, err
	}

	if sourceTimeout < time.Second {
		sourceTimeout = time.Second
	}

	server := &httpServer{
		sourceTimeout: sourceTimeout,
		tpl:           tpl,
	}

	r := mux.NewRouter()
	r.HandleFunc("/", server.indexHandler)
	r.HandleFunc("/api/add", server.addSourceHandler)

	fs := http.FileServer(http.Dir("./static/"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	srv := &http.Server{
		Addr:    address,
		Handler: r,
	}
	server.srv = srv

	return server, nil
}
