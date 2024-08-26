package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
)

// types for parsing API /models endpoint response
type ApiNamesResponse struct {
	Object string     `json:"object"`
	Data   []ApiModel `json:"data"`
}

type ApiModel struct {
	Id            string `json:"id"`
	Object        string `json:"object"`
	Created       int64  `json:"created"`
	OwnedBy       string `json:"owned_by"`
	Active        bool   `json:"active"`
	ContextWindow int    `json:"context_window"`
	PublicApps    any    `json:"public_apps"`
}

// populate the chat API select
func LoadApis(c echo.Context, app *pocketbase.PocketBase) error {
	var apiEditorParams []templates.ApiParams

	app.Dao().DB().
		Select("*").
		From("apis").
		OrderBy("name ASC").
		All(&apiEditorParams)

	userRecord, err := app.Dao().FindFirstRecordByData("users", "username", "default")
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve user record")
	}
	selectedApiId := userRecord.GetString("selected_api")

	c.Response().Writer.WriteHeader(200)
	modelSelect := templates.ApiSelect(selectedApiId, apiEditorParams)
	err = modelSelect.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render model select")
	}

	return nil
}

func ModelsUnavailableResponse(c echo.Context) error {
	c.Response().Writer.WriteHeader(200)
	noModels := templates.ApiModelsUnavailable()
	err := noModels.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve models from API")
	}

	return nil
}

// populate the chat model select
func LoadApiModels(modelId string, c echo.Context, app *pocketbase.PocketBase) error {
	apiRecord, err := app.Dao().FindRecordById("apis", modelId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve user record")
	}

	listModelsUrl := apiRecord.GetString("url") + "/models"
	request, err := http.NewRequest("GET", listModelsUrl, nil)
	request.Header.Add("Authorization", "Bearer "+apiRecord.GetString("api_key"))

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return ModelsUnavailableResponse(c)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {

		return c.String(http.StatusInternalServerError, "failed to read models endpoint response")
	}

	var data ApiNamesResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to read models endpoint response")
	}

	var modelNames []string
	for _, model := range data.Data {
		modelNames = append(modelNames, model.Id)
	}

	// set selected model in users table
	userRecord, err := app.Dao().FindFirstRecordByData("users", "username", "default")
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve user record for selected model name")
	}

	selectedModelName := userRecord.GetString("selected_model_name")

	c.Response().Writer.WriteHeader(200)
	modelNamesSelect := templates.ApiModelSelect(selectedModelName, modelNames)
	err = modelNamesSelect.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve models from API")
	}

	return nil
}

// open the API editor in the sidebar
func OpenApiEditor(c echo.Context, app *pocketbase.PocketBase) error {
	var apiEditorParams []templates.ApiParams

	app.Dao().DB().
		Select("*").
		From("apis").
		OrderBy("created DESC").
		All(&apiEditorParams)

	c.Response().Writer.WriteHeader(200)
	modelEditor := templates.ApiEditorsList(apiEditorParams)
	err := modelEditor.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render model editor")
	}

	return nil
}

// create new API definition
func CreateApi(c echo.Context, app *pocketbase.PocketBase) error {
	apisCollection, err := app.Dao().FindCollectionByNameOrId("apis")
	if err != nil {
		fmt.Println("error reading api DB")
	}

	newApiRecord := models.NewRecord(apisCollection)
	form := forms.NewRecordUpsert(app, newApiRecord)

	form.LoadData(map[string]any{
		"name":           "",
		"url":            "",
		"api_key":        "",
		"api_model_name": "",
	})
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to create new api DB record")
	}

	if err := form.Submit(); err != nil {
		fmt.Println("error creating new model")
		return c.String(http.StatusInternalServerError, "failed to create new api DB entry")
	}

	// if first API setup then automatically set user's selected_api id
	userRecord, err := app.Dao().FindFirstRecordByData("users", "username", "default")
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve user record to set API id")
	}

	if userRecord.GetString("selected_api") == "" {
		userRecord.Set("selected_api", newApiRecord.Id)
		if err := app.Dao().SaveRecord(userRecord); err != nil {
			return c.String(http.StatusInternalServerError, "failed to initialize user's selected API id")
		}
	}

	apiParams := templates.ApiParams{
		Id:           newApiRecord.Id,
		Name:         "",
		Url:          "",
		ApiKey:       "",
	}

	c.Response().Writer.WriteHeader(200)
	newModel := templates.NewApiEditor(apiParams)
	err = newModel.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render new api DB entry")
	}

	return nil
}

// update API definition
func UpdateApi(id string, data map[string]any, c echo.Context, app *pocketbase.PocketBase) error {
	apiRecord, err := app.Dao().FindRecordById("apis", id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to find api record")
	}

	apiRecord.Set("name", data["display-name"].(string))
	apiRecord.Set("url", data["url"].(string))
	apiRecord.Set("api_key", data["api-key"].(string))

	// set user selected api endpoint
	// set selected model in users table
	userRecord, err := app.Dao().FindFirstRecordByData("users", "username", "default")
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve user record")
	}

	userRecord.Set("selected_api", id)
	userRecord.Set("selected_model_name", "")
	if err := app.Dao().SaveRecord(userRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to update user record selected api")
	}

	if err := app.Dao().SaveRecord(apiRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to update api record")
	}

	c.Response().Header().Set("HX-Trigger", "refresh-apis")
	apiUpdateResult := templates.ModelUpdateResult()
	err = apiUpdateResult.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render api update result")
	}

	return nil
}

// set current API
func SelectApi(id string, selectedModel *string, c echo.Context, app *pocketbase.PocketBase) error {
	apiRecord, err := app.Dao().FindRecordById("apis", id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve selected api record")
	}

	// set selected model in users table
	userRecord, err := app.Dao().FindFirstRecordByData("users", "username", "default")
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve user record")
	}

	updated := false
	if userRecord.GetString("selected_api") != id {
		c.Response().Header().Set("HX-Trigger", "refresh-models")
		userRecord.Set("selected_model_name", "")
		updated = true
	}

	userRecord.Set("selected_api", id)
	if err := app.Dao().SaveRecord(userRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to update user record selected api")
	}

	// c.Response().Header().Set("HX-Trigger", "refresh-models")
	SelectApiStatus := templates.SelectApiStatus("Now chatting using " + apiRecord.GetString("name"), updated)
	err = SelectApiStatus.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render select api status")
	}

	return nil
}

func SelectModel(selectedModel string, c echo.Context, app *pocketbase.PocketBase) error {
	userRecord, err := app.Dao().FindFirstRecordByData("users", "username", "default")
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve user record")
	}

	updated := false
	if userRecord.GetString("selected_model_name") != selectedModel {
		updated = true
	}
	
	userRecord.Set("selected_model_name", selectedModel)
	if err := app.Dao().SaveRecord(userRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to update user record selected api")
	}

	SelectApiStatus := templates.SelectApiStatus("Now chatting with " + selectedModel, updated)
	err = SelectApiStatus.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render select model status")
	}

	return nil
}

// delete API definition
func DeleteApi(modelId string, c echo.Context, app *pocketbase.PocketBase) error {
	record, err := app.Dao().FindRecordById("apis", modelId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve api record for deletion")
	}

	if err := app.Dao().DeleteRecord(record); err != nil {
		return c.String(http.StatusInternalServerError, "failed to delete api record")
	}

	c.Response().Writer.WriteHeader(200)
	deletedApi := templates.DeletedModel()
	err = deletedApi.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render new api DB entry")
	}

	return nil
}
