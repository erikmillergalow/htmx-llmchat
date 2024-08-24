package main

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"

	"github.com/labstack/echo/v5"
)

func LoadThreadTags(id string, app *pocketbase.PocketBase) ([]templates.TagParams, error) {
	var threadTags []templates.TagParams
	threadRecord, err := app.Dao().FindRecordById("chat_meta", id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch thread record: %w", err)
	}
	if errs := app.Dao().ExpandRecord(threadRecord, []string{"tags"}, nil); len(errs) > 0 {
		return nil, fmt.Errorf("failed to expand thread tags")
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

func CreateTag(threadId string, c echo.Context, app *pocketbase.PocketBase) error {
	var allTagParams []templates.TagParams
	app.Dao().DB().
		Select("*").
		From("tags").
		OrderBy("value ASC").
		All(&allTagParams)

	c.Response().Writer.WriteHeader(200)
	tagEditor := templates.NewTagEditor(threadId, allTagParams)
	err := tagEditor.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render tag editor")
	}

	return nil
}

func SaveTag(threadId string, data map[string]any, c echo.Context, app *pocketbase.PocketBase) error {
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

	threadRecord, err := app.Dao().FindRecordById("chat_meta", threadId)
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
		ThreadId: threadId,
		Color:    color,
	}

	c.Response().Writer.WriteHeader(200)
	newTag := templates.NewTag(threadId, tagParams)
	err = newTag.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render tag editor")
	}

	return nil
}

func AddExistingTagToThread(threadId string, tagId string, c echo.Context, app *pocketbase.PocketBase) error {
	threadRecord, err := app.Dao().FindRecordById("chat_meta", threadId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to read thread DB")
	}

	tagExists := slices.Contains(threadRecord.GetStringSlice("tags"), tagId)

	threadRecord.Set("tags", append(threadRecord.GetStringSlice("tags"), tagId))
	if err = app.Dao().SaveRecord(threadRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to add existing tag to thread")
	}

	tagRecord, err := app.Dao().FindRecordById("tags", tagId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to find tag record to add to thread")
	}

	tagParams := templates.TagParams{
		Id:       tagRecord.Id,
		Value:    tagRecord.GetString("value"),
		ThreadId: threadId,
		Color:    tagRecord.GetString("color"),
	}

	c.Response().Writer.WriteHeader(200)

	if tagExists {
		noNewTag := templates.TagExists()
		err = noNewTag.Render(context.Background(), c.Response().Writer)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to render tag editor")
		}
	} else {
		newTag := templates.NewTag(threadId, tagParams)
		err = newTag.Render(context.Background(), c.Response().Writer)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to render tag editor")
		}
	}

	return nil
}

func OpenTagModifier(tagId string, threadId string, c echo.Context, app *pocketbase.PocketBase) error {
	tagRecord, err := app.Dao().FindRecordById("tags", tagId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to find tag record to open modifier")
	}

	tagParams := templates.TagParams{
		Id:       tagRecord.Id,
		Value:    tagRecord.GetString("value"),
		ThreadId: threadId,
		Color:    tagRecord.GetString("color"),
	}

	c.Response().Writer.WriteHeader(200)
	tagModifier := templates.TagModifier(tagParams, threadId)
	err = tagModifier.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render tag modifier")
	}

	return nil
}

func UpdateTag(tagId string, data map[string]any, c echo.Context, app *pocketbase.PocketBase) error {
	tagRecord, err := app.Dao().FindRecordById("tags", tagId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to find tag record to update")
	}

	tagRecord.Set("value", data["value"].(string))
	tagRecord.Set("color", data["color"].(string))

	if err := app.Dao().SaveRecord(tagRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to update tag record")
	}

	err = GetThreadList("creation", c, app)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch threads after tag deletion")
	}

	return nil
}

func DeleteTag(tagId string, c echo.Context, app *pocketbase.PocketBase) error {
	tagRecord, err := app.Dao().FindRecordById("tags", tagId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch tag to delete")
	}

	if err := app.Dao().DeleteRecord(tagRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to delete tag")
	}

	err = GetThreadList("creation", c, app)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch threads after tag deletion")
	}

	return nil
}

