package main

import (
	"fmt"
	"log"
	"os"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"

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

	// handle initial DB setup on first launch
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{Automigrate: true})

	selectedModel := "openai"

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		// serve static files from public dir
		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))

		var settings []templates.SideBarMenuParams
		app.Dao().DB().
			Select("*").
			From("settings").
			All(&settings)
		// chatgptClient := openai.NewClient(settings[0].OpenAIKey)

		e.Router.GET("/threads", func(c echo.Context) error {
			return GetThreadList("creation", c, app)
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
			fmt.Println(model)
			return SelectModel(model, &selectedModel, c, app)
		})

		e.Router.GET("/models", func(c echo.Context) error {
			return LoadModels(c, app)
		})

		e.Router.GET("/models/open", func(c echo.Context) error {
			return OpenModelEditor(c, app)
		})

		e.Router.POST("/models/create", func(c echo.Context) error {
			return CreateModel(c, app)
		})

		e.Router.POST("/models/update/:modelId", func(c echo.Context) error {
			id := c.PathParam("modelId")
			data := apis.RequestInfo(c).Data
			return UpdateModel(id, data, c, app)
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
			return OpenConfig(c, app)
		})

		e.Router.PUT("/config", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			return SaveConfig(data, c, app)
		})

		e.Router.GET("/config/done", func(c echo.Context) error {
			return GetThreadList("creation", c, app)
		})

		e.Router.GET("/sort/:method", func(c echo.Context) error {
			method := c.PathParam("method")
			return GetThreadList(method, c, app)
		})

		e.Router.GET("/search", func(c echo.Context) error {
			return OpenSearch(c, app)
		})

		e.Router.POST("/search", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			return Search(data, c, app)
		})

		// websocket connection:
		e.Router.GET("/ws", func(c echo.Context) error {
			return OpenChatSocket(&selectedModel, c, app)
		})

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
