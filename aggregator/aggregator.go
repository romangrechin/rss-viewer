package aggregator

import (
	"context"
	"github.com/mmcdole/gofeed"
	"github.com/romangrechin/rss-viewer/db"
	"log"
	"sync"
	"time"
)

type aggService struct {
	threads        int
	сhFeed         chan *db.ModelFeed
	сhSource       chan *db.ModelSource
	chSourceParser chan *db.ModelSource
	done           chan struct{}
	wg             sync.WaitGroup
	sourceTimeout  time.Duration
}

func (agg *aggService) Run() {
	agg.done = make(chan struct{})

	agg.сhFeed = make(chan *db.ModelFeed)
	agg.сhSource = make(chan *db.ModelSource)
	agg.chSourceParser = make(chan *db.ModelSource)

	agg.wg.Add(1)
	go agg.publishThread()

	for i := 0; i < agg.threads; i++ {
		agg.wg.Add(1)
		go agg.parserThread(i)
	}

	agg.wg.Add(1)
	go agg.loop()
}

func (agg *aggService) Stop() {
	if agg.done != nil {
		close(agg.done)

		agg.wg.Wait()

		close(agg.сhFeed)
		close(agg.сhSource)
		close(agg.chSourceParser)
	}
}

func (agg *aggService) CheckSourceIsValid(url string) error {
	parser := gofeed.NewParser()
	_, err := parser.ParseURLWithContext(url, context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (agg *aggService) parserThread(id int) {
	defer agg.wg.Done()
	parser := gofeed.NewParser()

	for {
		select {
		case <-agg.done:
			return
		case source := <-agg.chSourceParser:
			err := agg.parseFeeds(parser, source)
			if err != nil {
				log.Printf("SOURCE ERROR (%s): %v", source.Url, err)
			}
		}
	}
}

func (agg *aggService) parseFeeds(parser *gofeed.Parser, source *db.ModelSource) error {
	ctx, cancel := context.WithTimeout(context.Background(), agg.sourceTimeout)
	defer cancel()

	feeds, err := parser.ParseURLWithContext(source.Url, ctx)
	if err != nil {
		return err
	}

	var (
		sourceIsUpdated bool
		maxTime         time.Time
	)

	for _, item := range feeds.Items {
		// Пропускаем новость без заголовка
		if item.Title == "" {
			continue
		}

		// Получаем время последнего обновления или добавления новости.
		//Если его нет, устанавливаем текущее.
		feedTime := time.Now()
		if item.UpdatedParsed != nil {
			feedTime = *item.UpdatedParsed
		} else if item.PublishedParsed != nil {
			feedTime = *item.PublishedParsed
		}

		// Проверяем, что новость "свежая"
		if feedTime.After(source.LastTimeParsed) {
			if maxTime.Before(feedTime) {
				maxTime = feedTime
			}
			sourceIsUpdated = true

			feed := &db.ModelFeed{
				SourceId:    source.Id,
				Title:       item.Title,
				Description: item.Description,
				Datetime:    feedTime,
			}

			if len(item.Categories) > 0 {
				feed.Category = item.Categories[0]
			}

			if item.Image != nil {
				feed.ImageLink = item.Image.URL
			}

			feed.SourceLink = item.Link
			agg.сhFeed <- feed
		}
	}

	if sourceIsUpdated {
		source.LastTimeParsed = maxTime
		agg.сhSource <- source
	}
	return nil
}

func (agg *aggService) publishThread() {
	defer agg.wg.Done()

	var (
		sources []*db.ModelSource
		feeds   []*db.ModelFeed
	)

	publish := func() error {
		if len(sources)+len(feeds) == 0 {
			return nil
		}

		batch, err := db.NewBatch()
		if err != nil {
			return err
		}

		for _, source := range sources {
			batch.UpdateSource(source)
		}
		sources = nil

		for _, feed := range feeds {
			batch.InsertFeed(feed)
		}
		feeds = nil

		err = batch.Exec()
		if err == nil {
			return err
		}

		batch.Rollback()
		return err
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-agg.done:
			err := publish()
			if err != nil {
				log.Printf("DB ERROR (publish): %v", err)
			}
			return
		case <-ticker.C:
			// тут можно добавить ограничение на размер батча
			err := publish()
			if err != nil {
				log.Printf("DB ERROR (publish): %v", err)
			}
		case source := <-agg.сhSource:
			sources = append(sources, source)
		case feed := <-agg.сhFeed:
			feeds = append(feeds, feed)
		}
	}
}

func (agg *aggService) loop() {
	defer agg.wg.Done()

	offset := 0
	limit := agg.threads
	sleepTime := 5 * time.Second

	needExit := func() bool {
		select {
		case <-agg.done:
			return true
		default:
		}
		return false
	}

	startTime := time.Now()
	for {
		// Проверка, не остановлена ли программа
		if needExit() {
			return
		}

		offset = 0
		count, err := db.GetSourceCount()
		if err != nil {
			log.Printf("DB ERROR (source count): %v", err)
			time.Sleep(sleepTime)
			continue
		}

		for offset = 0; offset < count; offset += limit {
			if needExit() {
				return
			}

			sources, err := db.GetSources(limit, offset)
			if err != nil {
				log.Printf("DB ERROR (source count): %v", err)
				time.Sleep(sleepTime)
				continue
			}

			for _, source := range sources {
				source := source
				agg.chSourceParser <- source
			}
		}
		// Предотвращаем ДДОС RSS-ресурсов.
		endTime := time.Now()
		if endTime.Before(startTime.Add(sleepTime)) {
			time.Sleep(sleepTime)
		}
		startTime = time.Now()
	}
}

func New(threads int, sourceTimeout time.Duration) *aggService {
	if threads < 1 {
		threads = 1
		log.Println("WARNING: threads count must be > 0")
	}

	if sourceTimeout < time.Second {
		sourceTimeout = time.Second
		log.Println("WARNING: source timeout must be >= 1 sec.")
	}

	return &aggService{
		threads:       threads,
		sourceTimeout: sourceTimeout,
	}
}
