package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
)

func OpenSearch(c echo.Context, app *pocketbase.PocketBase) error {
	c.Response().Writer.WriteHeader(200)
	searchMenu := templates.SearchMenu()
	err := searchMenu.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render search menu")
	}

	return nil
}

func Search(data map[string]any, c echo.Context, app *pocketbase.PocketBase) error {
	fmt.Println("searching")
	searchValue := data["search-input"].(string)

	// var relevantMessageIds []string
	var relevantMessages []templates.LoadedMessageParams
	app.Dao().DB().
		Select("*").
		From("chat").
		Where(dbx.Like("message", searchValue)).
		OrderBy("created ASC").
		All(&relevantMessages)

	fmt.Println(relevantMessages)

	var relevantMessageIds []string
	for _, message := range relevantMessages {
		relevantMessageIds = append(relevantMessageIds, message.ThreadId)
	}

	fmt.Println(relevantMessageIds)

	var relevantThreadRecords, err = app.Dao().FindRecordsByIds("chat_meta", relevantMessageIds)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch threads relevant to search")
	}

	fmt.Println(relevantThreadRecords)

	// convert slice of records to struct templ expects, need to check for better ways to handle this...
	var relevantThreads []templates.ThreadListEntryParams
	for _, record := range relevantThreadRecords {
		thread := templates.ThreadListEntryParams{
			Id:                   record.GetString("id"),
			Title:                record.GetString("thread_title"),
			LastMessageTimestamp: record.GetDateTime("last_message_timestamp"),
			Created:              record.GetDateTime("created"),
		}
		relevantThreads = append(relevantThreads, thread)
	}

	var allTags [][]templates.TagParams
	for _, thread := range relevantThreads {
		threadTags, err := LoadThreadTags(thread.Id, c, app)
		if err != nil {
			return err
		}
		allTags = append(allTags, threadTags)
	}

	c.Response().Writer.WriteHeader(200)
	threadListEntries := templates.ThreadListEntries(relevantThreads, allTags)
	err = threadListEntries.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render threads relevant to search")
	}

	return nil
}
