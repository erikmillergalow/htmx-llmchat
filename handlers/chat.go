package handlers

import (
	"context"
	"net/http"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
)

func InitializeChat(c echo.Context, app *pocketbase.PocketBase) error {
	var apiEditorParams []templates.ApiParams

	app.Dao().DB().
		Select("*").
		From("apis").
		OrderBy("created DESC").
		All(&apiEditorParams)

	if len(apiEditorParams) == 0 {
		c.Response().Writer.WriteHeader(200)
		noApisInfo := templates.NoApisAvailable()
		err := noApisInfo.Render(context.Background(), c.Response().Writer)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to render no APIs info")
		}
	} else {
	  c.Response().Header().Set("HX-Trigger-After-Settle", "chat-window-loaded")
		c.Response().Writer.WriteHeader(200)
		chat := templates.ActiveChat()
		err := chat.Render(context.Background(), c.Response().Writer)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to render chat")
		}
	}

	return nil
}

func ToggleMessageUsefulness(messageId string, c echo.Context, app *pocketbase.PocketBase) error {
	messageRecord, err := app.Dao().FindRecordById("chat", messageId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to retrieve chat record to update usefulness")
	}

	useful := messageRecord.GetBool("useful")
	messageRecord.Set("useful", !useful)
	if err = app.Dao().SaveRecord(messageRecord); err != nil {
		return c.String(http.StatusInternalServerError, "failed to update chat record's usefulness")
	}

	c.Response().Writer.WriteHeader(200)
	usefulnessResponse := templates.UsefulnessResponse(messageId, !useful)
	err = usefulnessResponse.Render(context.Background(), c.Response().Writer)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to render usefulness response")
	}

	return nil
}

