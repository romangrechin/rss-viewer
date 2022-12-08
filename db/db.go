package db

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v4/pgxpool"
	"time"
)

type dbHandler struct {
	pool *pgxpool.Pool
}

var (
	db                *dbHandler
	defaultSourceTime time.Time
)

var (
	ErrDatabaseNotInit          = errors.New("database is not initialized")
	ErrDatabaseAlreadyConnected = errors.New("database is already connected")
)

func init() {
	defaultSourceTime, _ = time.Parse("2006-01-02 15:04:05", "0001-01-01 00:00:00")
}

func Connect(connString string) error {
	if db != nil {
		return ErrDatabaseAlreadyConnected
	}
	pool, err := pgxpool.Connect(context.Background(), connString)
	if err != nil {
		return err
	}

	err = pool.Ping(context.Background())
	if err != nil {
		pool.Close()
		return err
	}

	db = &dbHandler{
		pool: pool,
	}
	return nil
}

func Close() error {
	if db == nil {
		return ErrDatabaseNotInit
	}
	db.pool.Close()
	return nil
}

func GetSourceCount() (int, error) {
	if db == nil {
		return 0, ErrDatabaseNotInit
	}

	row := db.pool.QueryRow(context.Background(), "SELECT COUNT(id) FROM sources")
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func GetSources(limit, offset int) ([]*ModelSource, error) {
	if db == nil {
		return nil, ErrDatabaseNotInit
	}

	rows, err := db.pool.Query(context.Background(), "SELECT id, url, last_time_parsed FROM sources ORDER BY id LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []*ModelSource
	for rows.Next() {
		source := &ModelSource{}
		err := rows.Scan(&source.Id, &source.Url, &source.LastTimeParsed)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func IsSourceExists(url string) (bool, error) {
	if db == nil {
		return false, ErrDatabaseNotInit
	}

	row := db.pool.QueryRow(context.Background(), "SELECT COUNT(id) FROM sources WHERE url=$1", url)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return false, err
	}
	return count != 0, nil
}

func InsertSource(url string) (*ModelSource, error) {
	if db == nil {
		return nil, ErrDatabaseNotInit
	}

	var id int

	row := db.pool.QueryRow(context.Background(), "INSERT INTO sources (url, last_time_parsed) VALUES ($1, $2) RETURNING id", url, defaultSourceTime)
	err := row.Scan(&id)
	if err != nil {
		return nil, err
	}

	return &ModelSource{
		Id:             id,
		Url:            url,
		LastTimeParsed: defaultSourceTime,
	}, nil
}

func GetFeedCount(search string) (int, error) {
	if db == nil {
		return 0, ErrDatabaseNotInit
	}

	var (
		args  []interface{}
		query = "SELECT COUNT(id) FROM feeds"
	)

	if search != "" {
		query += " WHERE title ILIKE '%' || $1 || '%' OR description ILIKE '%' || $1 || '%'"
		args = append(args, search)
	}

	row := db.pool.QueryRow(context.Background(), query, args...)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func GetFeeds(search string, limit, offset int) ([]*ModelFeed, error) {
	if db == nil {
		return nil, ErrDatabaseNotInit
	}

	var (
		args  = []interface{}{limit, offset}
		query = "SELECT id, title, image_link, source_link, category, description, datetime, source_id FROM feeds"
	)

	if search != "" {
		query += " WHERE title ILIKE '%' || $3 || '%' OR description ILIKE '%' || $3 || '%'"
		args = append(args, search)
	}
	query += " ORDER BY datetime DESC LIMIT $1 OFFSET $2"

	rows, err := db.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []*ModelFeed
	for rows.Next() {
		feed := &ModelFeed{}
		err := rows.Scan(&feed.Id, &feed.Title, &feed.ImageLink, &feed.SourceLink, &feed.Category, &feed.Description, &feed.Datetime, &feed.SourceId)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, feed)
	}
	return feeds, nil
}
