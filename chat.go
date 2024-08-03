package main

import (
	"context"
	"fmt"
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

	c.Response().Writer.WriteHeader(200)
	fmt.Println("len apis")
	fmt.Println(len(apiEditorParams))
	if len(apiEditorParams) == 0 {
		noApisInfo := templates.NoApisAvailable()
		err := noApisInfo.Render(context.Background(), c.Response().Writer)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to render no APIs info")
		}
	} else {
		chat := templates.ActiveChat()
		err := chat.Render(context.Background(), c.Response().Writer)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to render chat")
		}
	}

	return nil
}
