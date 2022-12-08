package db

import "time"

type ModelSource struct {
	Id             int
	Url            string
	LastTimeParsed time.Time
}

type ModelFeed struct {
	Id          int
	SourceId    int
	Title       string
	Description string
	Category    string
	SourceLink  string
	ImageLink   string
	Datetime    time.Time
}
