package db

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v4"
	"log"
)

var (
	ErrTxClosed = errors.New("transaction closed")
)

type dbBatch struct {
	tx    pgx.Tx
	batch *pgx.Batch
	ctx   context.Context
}

func (b *dbBatch) InsertFeed(feed *ModelFeed) {
	b.batch.Queue("INSERT INTO feeds (title,description,category,image_link,source_link,datetime,source_id) VALUES ($1, $2, $3, $4,$5,$6,$7)",
		feed.Title,
		feed.Description,
		feed.Category,
		feed.ImageLink,
		feed.SourceLink,
		feed.Datetime,
		feed.SourceId)
}

func (b *dbBatch) UpdateSource(source *ModelSource) {
	b.batch.Queue("UPDATE sources SET last_time_parsed=$1 WHERE id=$2",
		source.LastTimeParsed, source.Id)
}

func (b *dbBatch) Exec() error {
	tx, err := db.pool.Begin(b.ctx)
	if err != nil {
		log.Println("BEGIN")
		return err
	}
	b.tx = tx
	br := tx.SendBatch(b.ctx, b.batch)
	_, err = br.Exec()
	if err != nil {
		br.Close()
		return err
	}
	br.Close()

	return tx.Commit(b.ctx)
}

func (b *dbBatch) Rollback() error {
	if b.tx == nil {
		return ErrTxClosed
	}

	return b.tx.Rollback(b.ctx)
}

func NewBatch() (*dbBatch, error) {
	if db == nil {
		return nil, ErrDatabaseNotInit
	}

	return &dbBatch{
		batch: &pgx.Batch{},
		ctx:   context.Background(),
	}, nil
}
