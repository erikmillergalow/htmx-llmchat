package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
)

func GetThreadList(c echo.Context, app *pocketbase.PocketBase) error {
	var threads []templates.ThreadListEntryParams
	app.Dao().DB().Select("*").From("chat_meta").OrderBy("created DESC").All(&threads)

	var allTags [][]templates.TagParams

	// load thread tags
	for _, thread := range threads {
		var threadTags []templates.TagParams
		threadRecord, err := app.Dao().FindRecordById("chat_meta", thread.Id)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to fetch thread record")
		}
		if errs := app.Dao().ExpandRecord(threadRecord, []string{"tags"}, nil); len(errs) > 0 {
			return c.String(http.StatusInternalServerError, "failed to expand thread tags")
		}
		for _, expandedTag := range threadRecord.ExpandedAll("tags") {
			fmt.Println(expandedTag)
			threadTags = append(threadTags, templates.TagParams{
				Value:    expandedTag.GetString("value"),
				ThreadId: expandedTag.GetString("value"),
				Color:    expandedTag.GetString("color"),
				Id:       expandedTag.Id,
			})
		}
		allTags = append(allTags, threadTags)
	}

	c.Response().Writer.WriteHeader(200)
	threadListEntry := templates.ThreadListEntries(threads, allTags)
	fmt.Println(threadListEntry)
	err := threadListEntry.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render thread list repsonse")
	}

	return nil
}

func GetThread(id string, c echo.Context, app *pocketbase.PocketBase) error {
	var messages []templates.LoadedMessageParams
	app.Dao().DB().
		Select("*").
		From("chat").
		Where(dbx.NewExp("thread_id = {:id}", dbx.Params{"id": id})).
		All(&messages)

	c.Response().Writer.WriteHeader(200)
	loadedChat := templates.LoadedThread(messages)
	err := loadedChat.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render loaded chat response")
	}

	return nil
}

func CreateThread(c echo.Context, app *pocketbase.PocketBase) error {
	threadsCollection, err := app.Dao().FindCollectionByNameOrId("chat_meta")
	if err != nil {
		fmt.Println("error reading threads DB")
	}

	newThreadRecord := models.NewRecord(threadsCollection)
	form := forms.NewRecordUpsert(app, newThreadRecord)

	form.LoadData(map[string]any{
		"last_message":           "Empty chat...",
		"last_message_timestamp": newThreadRecord.Created,
	})

	if err := form.Submit(); err != nil {
		fmt.Println("error creating new thread")
		return c.String(http.StatusInternalServerError, "failed to create new thread DB entry")
	}

	newThreadRecord.Set("thread_title", newThreadRecord.Id)
	if err := app.Dao().SaveRecord(newThreadRecord); err != nil {
		fmt.Println("error creating new thread")
		return c.String(http.StatusInternalServerError, "failed to set thread title to record ID")
	}

	threadParams := templates.ThreadListEntryParams{
		Id:                   newThreadRecord.Id,
		Title:                newThreadRecord.Id,
		LastMessage:          "Empty chat...",
		LastMessageTimestamp: newThreadRecord.Created,
		Created:              newThreadRecord.Created,
	}

	// return thread, target threads-list, swap beforestart (or whatever the top is
	c.Response().Writer.WriteHeader(200)
	newThread := templates.NewThreadListEntry(threadParams)
	err = newThread.Render(context.Background(), c.Response().Writer)
	if err != nil {
		fmt.Printf("Error rendering new thread: %v\n", err)
		return c.String(http.StatusInternalServerError, "failed to render new thread DB entry")
	}
	return nil
}

func EditThreadTitle(id string, c echo.Context, app *pocketbase.PocketBase) error {
	threadRecord, err := app.Dao().FindRecordById("chat_meta", id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to get thread record to set title")
	}

	c.Response().Writer.WriteHeader(200)
	titleEditor := templates.ThreadTitleEditor(id, threadRecord.GetString("thread_title"))
	err = titleEditor.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render title editor")
	}

	return nil
}

func SaveThreadTitle(id string, title string, c echo.Context, app *pocketbase.PocketBase) error {
	idRecord, err := app.Dao().FindRecordById("chat_meta", id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to find thread record")
	}
	if title != "" {
		idRecord.Set("thread_title", title)
		if err := app.Dao().SaveRecord(idRecord); err != nil {
			return c.String(http.StatusInternalServerError, "failed to update thread title record")
		}
	} else {
		title = idRecord.GetString("thread_title")
	}
	threadTitle := templates.ThreadTitleUpdate(id, title)
	err = threadTitle.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render thread title")
	}

	return nil
}