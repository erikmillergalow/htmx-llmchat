package main

import (
	"log"
	"os"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	openai "github.com/sashabaranov/go-openai"

	_ "github.com/erikmillergalow/htmx-llmchat/migrations"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

var (
	upgrader = websocket.Upgrader{}
)

func main() {
	app := pocketbase.New()

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{Automigrate: true})

	selectedModel := "openai"

	// serve static files from public dir
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))

		var settings []templates.SideBarMenuParams
		app.Dao().DB().
			Select("*").
			From("settings").
			All(&settings)
		chatgptClient := openai.NewClient(settings[0].OpenAIKey)

		// begin endpoints
		e.Router.GET("/threads", func(c echo.Context) error {
			return GetThreadList(c, app)
		})

		e.Router.GET("/thread/:id", func(c echo.Context) error {
			threadId := c.PathParam("id")
			return GetThread(threadId, c, app)
		})

		e.Router.GET("/thread/title/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			return EditThreadTitle(id, c, app)
		})

		e.Router.PUT("/thread/title/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			data := apis.RequestInfo(c).Data
			title := data["title"].(string)

			return SaveThreadTitle(id, title, c, app)
		})

		e.Router.POST("/thread/create", func(c echo.Context) error {
			return CreateThread(c, app)
		})

		e.Router.GET("/model", func(c echo.Context) error {
			model := c.QueryParam("model")
			return SelectModel(model, &selectedModel, c, app)
		})

		e.Router.GET("/thread/tag/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			return CreateTag(id, c)
		})

		e.Router.POST("/thread/tag/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			data := apis.RequestInfo(c).Data
			return SaveTag(id, data, c, app)

		})

		e.Router.GET("/config", func(c echo.Context) error {
			return GetConfig(c, app)
		})

		e.Router.PUT("/config", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			return SaveConfig(data, c, app)
		})

		e.Router.GET("/config/done", func(c echo.Context) error {
			return GetThreadList(c, app)
		})

		// websocket connection:
		e.Router.GET("/ws", func(c echo.Context) error {
			return OpenChatSocket(&selectedModel, chatgptClient, c, app)
		})

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
