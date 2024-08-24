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
	"github.com/pocketbase/pocketbase/tools/list"
)

func collectAllThreads(sortBy string, app *pocketbase.PocketBase) ([]templates.ThreadListEntryParams, [][]templates.TagParams, error) {
	var threads []templates.ThreadListEntryParams
	app.Dao().DB().
		Select("*").
		From("chat_meta").
		OrderBy(sortBy).
		All(&threads)

	var tags [][]templates.TagParams

	// load thread tags
	for _, thread := range threads {
		threadTags, err := LoadThreadTags(thread.Id, app)
		if err != nil {
			return nil, nil, err
		}
		tags = append(tags, threadTags)
	}

	return threads, tags, nil
}

func GetThreadList(sortMethod string, c echo.Context, app *pocketbase.PocketBase) error {
	
	sortBy := "created DESC"
	switch sortMethod {
	case "creation":
		sortBy = "created DESC"
	case "interaction":
		sortBy = "last_message_timestamp DESC"
	case "az":
		sortBy = "thread_title ASC"
	default:
		sortBy = "created DESC"
	}

	threads, allTags, err := collectAllThreads(sortBy, app)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch all threads")
	}

	c.Response().Writer.WriteHeader(200)
	threadListEntry := templates.ThreadListEntries(threads, allTags)
	fmt.Println(threadListEntry)
	err = threadListEntry.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render thread list response")
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

	threadRecord, err := app.Dao().FindRecordById("chat_meta", id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch thread record for loading thread title")
	}

	c.Response().Header().Set("HX-Trigger-After-Settle", "format-thread-markdown")
	c.Response().Writer.WriteHeader(200)
	loadedChat := templates.LoadedThread(threadRecord.GetString("thread_title"), messages)
	err = loadedChat.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render loaded chat response")
	}

	return nil
}

func DeleteThread(threadId string, c echo.Context, app *pocketbase.PocketBase) error {
	threadRecord, err := app.Dao().FindRecordById("chat_meta", threadId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch thread record to delete")
	}

	if err := app.Dao().DeleteRecord(threadRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch thread record to delete")
	}
	
	deleteQuery := "DELETE FROM chat WHERE thread_id ='" + threadId + "'"
	fmt.Println(deleteQuery)
	if _, err := app.Dao().DB().NewQuery(deleteQuery).Execute(); err != nil {
		return c.String(http.StatusInternalServerError, "failed to delete chat messages with thread")
	}

	c.Response().Header().Set("HX-Trigger-After-Settle", "chat-window-loaded")
	c.Response().Writer.WriteHeader(200)
	deleteMessage := templates.DeleteThreadMessage()
	err = deleteMessage.Render(context.Background(), c.Response().Writer)
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

	newThreadList, tags, err := collectAllThreads("created DESC", app)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch all threads while creating new thread")
	}
	fmt.Println(newThreadList)

	c.Response().Header().Set("HX-Trigger-After-Settle", "chat-window-loaded")
	c.Response().Writer.WriteHeader(200)
	updatedThreadList := templates.NewThreadListEntries(newThreadRecord.Id, newThreadList, tags)
	err = updatedThreadList.Render(context.Background(), c.Response().Writer)
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

func RemoveTagFromThread(threadId string, tagId string, c echo.Context, app *pocketbase.PocketBase) error {
	threadRecord, err := app.Dao().FindRecordById("chat_meta", threadId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to find thread record to remove tag")
	}

	threadRecord.Set("tags", list.SubtractSlice(threadRecord.GetStringSlice("tags"), []string{tagId}))
	if err := app.Dao().SaveRecord(threadRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to update thread title record")
	}
	
	return nil
}
