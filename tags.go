package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"

	"github.com/labstack/echo/v5"
)

func LoadThreadTags(id string, c echo.Context, app *pocketbase.PocketBase) ([]templates.TagParams, error) {
	var threadTags []templates.TagParams
	threadRecord, err := app.Dao().FindRecordById("chat_meta", id)
	if err != nil {
		return nil, c.String(http.StatusInternalServerError, "failed to fetch thread record")
	}
	if errs := app.Dao().ExpandRecord(threadRecord, []string{"tags"}, nil); len(errs) > 0 {
		return nil, c.String(http.StatusInternalServerError, "failed to expand thread tags")
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
	return threadTags, nil
}

func CreateTag(threadId string, c echo.Context) error {
	c.Response().Writer.WriteHeader(200)
	tagEditor := templates.NewTagEditor(threadId)
	err := tagEditor.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render tag editor")
	}

	return nil
}

func SaveTag(id string, data map[string]any, c echo.Context, app *pocketbase.PocketBase) error {
	value := data["value"].(string)
	color := data["color"].(string)

	tagsCollection, err := app.Dao().FindCollectionByNameOrId("tags")
	if err != nil {
		return c.String(http.StatusInternalServerError, "error reading tags DB")
	}

	newTagRecord := models.NewRecord(tagsCollection)
	form := forms.NewRecordUpsert(app, newTagRecord)

	form.LoadData(map[string]any{
		"value": value,
		"color": color,
	})

	if err := form.Submit(); err != nil {
		return c.String(http.StatusInternalServerError, "failed to create new tag DB entry")
	}

	threadRecord, err := app.Dao().FindRecordById("chat_meta", id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to read thread DB")
	}

	threadRecord.Set("tags", append(threadRecord.GetStringSlice("tags"), newTagRecord.Id))
	if err = app.Dao().SaveRecord(threadRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to add tag to thread")
	}

	tagParams := templates.TagParams{
		Id:       newTagRecord.Id,
		Value:    value,
		ThreadId: id,
		Color:    color,
	}

	c.Response().Writer.WriteHeader(200)
	newTag := templates.NewTag(tagParams)
	err = newTag.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render tag editor")
	}

	return nil
}
