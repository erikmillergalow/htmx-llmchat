package main

import (
	"context"
	"net/http"
	"sort"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
)

func OpenConfig(c echo.Context, app *pocketbase.PocketBase) error {
	// this should be associated with user accounts for server style setup
	var settings []templates.SideBarMenuParams
	app.Dao().DB().
		Select("*").
		From("settings").
		All(&settings)

	c.Response().Writer.WriteHeader(200)
	loadedSettingsMenu := templates.SideBarMenu(settings[0])
	err := loadedSettingsMenu.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render loaded chat response")
	}

	return nil
}

func SaveConfig(data map[string]any, c echo.Context, app *pocketbase.PocketBase) error {
	// this should be associated with user accounts for server style setup
	settingsRecord, err := app.Dao().FindFirstRecordByData("settings", "type", "keys")
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to fetch keys record")
	}

	openAIKey := data["openai-key"].(string)
	groqKey := data["groq-key"].(string)

	settingsRecord.Set("openai_key", openAIKey)
	settingsRecord.Set("groq_key", groqKey)
	if err = app.Dao().SaveRecord(settingsRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to save key settings")
	}

	c.Response().Writer.WriteHeader(200)
	settingsUpdated := templates.SettingsUpdated()
	err = settingsUpdated.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render settings update response")
	}

	return nil
}

func GetModelStats(c echo.Context, app *pocketbase.PocketBase) error {
	var messages []templates.LoadedMessageParams
	app.Dao().DB().
		Select("*").
		From("chat").
		All(&messages)

	totalMessages := make(map[string]int)
	usefulMessages := make(map[string]int)
	for _, message := range messages {
		totalMessages[message.Model]++
		if message.Useful {
			usefulMessages[message.Model]++
		}
	}

	var sortedKeys []string
	percent := make(map[string]float64)
	for model := range totalMessages {
		sortedKeys = append(sortedKeys, model)
		if _, ok := usefulMessages[model]; !ok {
			usefulMessages[model] = 0
			percent[model] = 0.0
		} else {
			percent[model] = float64(usefulMessages[model]) / float64(totalMessages[model])
		}
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		return percent[sortedKeys[i]] > percent[sortedKeys[j]]
	})
		
	c.Response().Writer.WriteHeader(200)
	modelStatsViewer := templates.ModelStatsViewer(sortedKeys, totalMessages, usefulMessages, percent)
	err := modelStatsViewer.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render settings update response")
	}

	return nil
}
