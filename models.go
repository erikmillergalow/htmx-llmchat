package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
)

func OpenModelEditor(c echo.Context, app *pocketbase.PocketBase) error {
	var modelEditorParams []templates.ModelParams

	app.Dao().DB().
		Select("*").
		From("models").
		OrderBy("created DESC").
		All(&modelEditorParams)

	c.Response().Writer.WriteHeader(200)
	modelEditor := templates.ModelEditorsList(modelEditorParams)
	err := modelEditor.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render model editor")
	}

	return nil
}

func CreateModel(c echo.Context, app *pocketbase.PocketBase) error {
	fmt.Println("New model!")
	modelsCollection, err := app.Dao().FindCollectionByNameOrId("models")
	if err != nil {
		fmt.Println("error reading models DB")
	}

	newModelRecord := models.NewRecord(modelsCollection)
	form := forms.NewRecordUpsert(app, newModelRecord)

	form.LoadData(map[string]any{
		"name":           "",
		"url":            "",
		"api_key":        "",
		"api_model_name": "",
		"color":          "",
	})
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to create new models DB record")
	}

	if err := form.Submit(); err != nil {
		fmt.Println("error creating new model")
		return c.String(http.StatusInternalServerError, "failed to create new model DB entry")
	}

	modelParams := templates.ModelParams{
		Id:           newModelRecord.Id,
		Name:         "",
		Url:          "",
		ApiKey:       "",
		ApiModelName: "",
		Color:        "",
	}

	c.Response().Writer.WriteHeader(200)
	newModel := templates.NewModelEditor(modelParams)
	err = newModel.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render new model DB entry")
	}

	return nil
}

func UpdateModel(id string, data map[string]any, c echo.Context, app *pocketbase.PocketBase) error {
	modelRecord, err := app.Dao().FindRecordById("models", id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to find model record")
	}

	modelRecord.Set("name", data["display-name"].(string))
	modelRecord.Set("url", data["url"].(string))
	modelRecord.Set("api_key", data["api-key"].(string))
	modelRecord.Set("api_model_name", data["api-model-name"].(string))
	modelRecord.Set("color", data["color"].(string))

	if err := app.Dao().SaveRecord(modelRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to update model record")
	}

	modelUpdateResult := templates.ModelUpdateResult()
	err = modelUpdateResult.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render model update result")
	}

	return nil
}

func SelectModel(model string, selectedModel *string, c echo.Context, app *pocketbase.PocketBase) error {
	if model == "openai" {
		c.Response().Writer.WriteHeader(200)
		selectModelStatus := templates.SelectModelStatus("Now chatting with OpenAI")
		*selectedModel = "openai"
		err := selectModelStatus.Render(context.Background(), c.Response().Writer)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to render select model status")
		}
	} else if model == "groq" {
		c.Response().Writer.WriteHeader(200)
		selectModelStatus := templates.SelectModelStatus("Now chatting with Groq API")
		err := selectModelStatus.Render(context.Background(), c.Response().Writer)
		*selectedModel = "groq"
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to render select model status")
		}

	} else {
		return c.String(http.StatusInternalServerError, "model not recognized")
	}

	return nil
}
