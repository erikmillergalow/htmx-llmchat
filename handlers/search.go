package handlers

import (
	"context"
	"net/http"
	"slices"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
)

func OpenSearch(c echo.Context, app *pocketbase.PocketBase) error {
	var allTagParams []templates.TagParams
	app.Dao().DB().
		Select("*").
		From("tags").
		OrderBy("created DESC").
		All(&allTagParams)

	var allChatParams []templates.LoadedMessageParams
	app.Dao().DB().
		Select("*").
		From("chat").
		OrderBy("created DESC").
		All(&allChatParams)

	var usedModels []string
	for _, message := range allChatParams {
		if !slices.Contains(usedModels, message.Model) {
			usedModels = append(usedModels, message.Model)
		}
	}

	c.Response().Writer.WriteHeader(200)
	searchMenu := templates.SearchMenu(allTagParams, usedModels)
	err := searchMenu.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render search menu")
	}

	return nil
}

func Search(data map[string]any, c echo.Context, app *pocketbase.PocketBase) error {
	searchValue := data["search-input"].(string)
	tagFilter := data["tag"].(string)
	modelFilter := data["model"].(string)
	usefulFilter := data["useful"].(string)

	var relevantMessages []templates.LoadedMessageParams
	query := app.Dao().DB().
		Select("*").
		From("chat")

	if usefulFilter == "useful" {
		query = query.Where(dbx.NewExp("useful = true"))
	} else if usefulFilter == "not-useful" {
		query = query.Where(dbx.NewExp("thread_id NOT IN (SELECT DISTINCT thread_id FROM chat WHERE useful = true)"))
	}

	if searchValue != "" {
		query = query.AndWhere(dbx.Like("message", searchValue))
	}

	query.All(&relevantMessages)

	var relevantMessageIds []string
	for _, message := range relevantMessages {
		if modelFilter == "any" || modelFilter == message.Model {
			relevantMessageIds = append(relevantMessageIds, message.ThreadId)
		}
	}

	var relevantThreadRecords, err = app.Dao().FindRecordsByIds("chat_meta", relevantMessageIds)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch threads relevant to search")
	}


	// convert slice of records to struct templ expects, need to check for better ways to handle this...
	var relevantThreads []templates.ThreadListEntryParams
	for _, record := range relevantThreadRecords {
		if tagFilter == "any" || slices.Contains(record.GetStringSlice("tags"), tagFilter) {
			thread := templates.ThreadListEntryParams{
				Id:                   record.GetString("id"),
				Title:                record.GetString("thread_title"),
				LastMessageTimestamp: record.GetDateTime("last_message_timestamp"),
				Created:              record.GetDateTime("created"),
			}
			relevantThreads = append(relevantThreads, thread)
		}
	}

	var allTags [][]templates.TagParams
	for _, thread := range relevantThreads {
		threadTags, err := LoadThreadTags(thread.Id, app)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to fetch thread tags")
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
