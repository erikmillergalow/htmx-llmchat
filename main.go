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

		e.Router.GET("/chat", func(c echo.Context) error {
			fmt.Println("initializing chat")
			return InitializeChat(c, app)
		})

		e.Router.POST("/chat/useful/:messageId", func(c echo.Context) error {
			messageId := c.PathParam("messageId")
			return ToggleMessageUsefulness(messageId, c, app)
		})

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

		e.Router.DELETE("/thread/:threadId", func(c echo.Context) error {
			id := c.PathParam("threadId")
			return DeleteThread(id, c, app)
		})
		
		// load APIs for chat window dropdown
		e.Router.GET("/apis", func(c echo.Context) error {
			return LoadApis(c, app)
		})

		// select an API from chat window dropdown
		e.Router.PUT("/api/select", func(c echo.Context) error {
			//model := c.QueryParam("model")
			data := apis.RequestInfo(c).Data
			id := data["api"].(string)
			return SelectApi(id, &selectedModel, c, app)
		})

		// attempt to load list of models an API provides
		e.Router.POST("/apis/models", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			id := data["api"].(string)
			return LoadApiModels(id, c, app)
		})

		// select a model from chat window dropdown
		e.Router.POST("/apis/model", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			name := data["api-model-name"].(string)
			return SelectModel(name, c, app)
		})

		// open API editor in the sidebar
		e.Router.GET("/apis/open", func(c echo.Context) error {
			return OpenApiEditor(c, app)
		})

		// create new API in the sidebar
		e.Router.POST("/apis/create", func(c echo.Context) error {
			return CreateApi(c, app)
		})

		// delete an API in the sidebar
		e.Router.DELETE("/apis/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			return DeleteApi(id, c, app)
		})

		// update existing API in the sidebar
		e.Router.PATCH("/apis/update/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			data := apis.RequestInfo(c).Data
			return UpdateApi(id, data, c, app)
		})

		e.Router.GET("/thread/tag/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			return CreateTag(id, c, app)
		})

		e.Router.POST("/thread/tag/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			data := apis.RequestInfo(c).Data
			return SaveTag(id, data, c, app)
		})

		// open editor to update tag, remove from thread, or delete entirely
		e.Router.GET("/tag/:tagId/thread/:threadId", func(c echo.Context) error {
			tagId := c.PathParam("tagId")
			threadId := c.PathParam("threadId")
			return OpenTagModifier(tagId, threadId, c, app)
		})

		e.Router.POST("/tag/update/:tagId", func(c echo.Context) error {
			tagId := c.PathParam("tagId")
			data := apis.RequestInfo(c).Data
			return UpdateTag(tagId, data, c, app)
		})

		e.Router.DELETE("/tag/:tagId", func(c echo.Context) error {
			tagId := c.PathParam("tagId")
			return DeleteTag(tagId, c, app)
		})

		// remove tag from thread
		e.Router.DELETE("/thread/:threadId/tag/:tagId", func(c echo.Context) error {
			threadId := c.PathParam("threadId")
			tagId := c.PathParam("tagId")
			return RemoveTagFromThread(threadId, tagId, c, app)
		})

		// add tag to thread
		e.Router.POST("/thread/:threadId/tag/:tagId", func(c echo.Context) error {
			threadId := c.PathParam("threadId")
			tagId := c.PathParam("tagId")
			return AddExistingTagToThread(threadId, tagId, c, app)
		})

		// open config
		e.Router.GET("/config", func(c echo.Context) error {
			return OpenConfig(c, app)
		})

		e.Router.GET("/stats", func(c echo.Context) error {
			return GetModelStats(c, app)
		})
		// update config
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
