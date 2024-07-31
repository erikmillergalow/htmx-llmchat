package main

import (
	"context"
	"net/http"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
)

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
