package web

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mmcdole/gofeed"
	"github.com/romangrechin/rss-viewer/db"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	PageSize = 30
)

var ErrOK = errors.New("OK")

type jsonResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type htmlResponse struct {
	Page     int
	Count    int
	PrevPage string
	NextPage string
	Feeds    []*db.ModelFeed // Плохая практика использовать модель БД в интерфейсе((
}

func writeJsonResponse(w http.ResponseWriter, code int, err error) {
	if err == nil {
		err = ErrOK
	}

	resp := &jsonResponse{
		Code:    code,
		Message: err.Error(),
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

func (hs *httpServer) addSourceHandler(w http.ResponseWriter, r *http.Request) {
	url := strings.TrimSpace(r.FormValue("url"))
	if url == "" {
		writeJsonResponse(w, http.StatusBadRequest, errors.New("url can not be empty"))
		return
	}

	exists, err := db.IsSourceExists(url)
	if err != nil {
		writeJsonResponse(w, http.StatusInternalServerError, err)
		return
	}

	if exists {
		writeJsonResponse(w, http.StatusOK, errors.New("url already exist"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), hs.sourceTimeout)
	defer cancel()

	parser := gofeed.NewParser()
	_, err = parser.ParseURLWithContext(url, ctx)
	if err != nil {
		writeJsonResponse(w, http.StatusBadRequest, err)
		return
	}

	_, err = db.InsertSource(url)
	if err != nil {
		writeJsonResponse(w, http.StatusInternalServerError, err)
		return
	}

	writeJsonResponse(w, http.StatusOK, nil)
}

func (hs *httpServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	var (
		page int = 1
		err  error
	)

	search := strings.TrimSpace(r.FormValue("q"))
	pageStr := r.FormValue("p")

	if pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	count, err := db.GetFeedCount(search)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("DB ERROR: (feed count)", err)
		return
	}

	resp := &htmlResponse{
		Page: page,
	}

	if count > 0 {
		val := url.Values{}
		if search != "" {
			val.Add("q", search)
		}

		totalPages := int(math.Ceil(float64(count) / float64(PageSize)))
		if totalPages < page {
			val.Set("p", strconv.Itoa(totalPages))
			path := "/?" + val.Encode()
			http.Redirect(w, r, path, http.StatusFound)
			return
		}

		offset := page - 1
		feeds, err := db.GetFeeds(search, PageSize, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("DB ERROR: (get feeds)", err)
			return
		}
		resp.Feeds = feeds
		resp.Count = len(feeds)

		if page > 1 {
			val.Set("p", strconv.Itoa(page-1))
			resp.PrevPage = "/?" + val.Encode()
		}

		if page < totalPages {
			val.Set("p", strconv.Itoa(page+1))
			resp.NextPage = "/?" + val.Encode()
		}
	}

	w.Header().Add("Content Type", "text/html")
	hs.tpl.Execute(w, resp)
}
