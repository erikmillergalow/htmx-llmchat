package main

import (
	"log"
	"os"

	"github.com/erikmillergalow/htmx-llmchat/templates"
	"github.com/erikmillergalow/htmx-llmchat/handlers"

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

		// initialize chat window if an API is available
		e.Router.GET("/chat", func(c echo.Context) error {
			return handlers.InitializeChat(c, app)
		})

		// toggle chat message usefulness tag
		e.Router.POST("/chat/useful/:messageId", func(c echo.Context) error {
			messageId := c.PathParam("messageId")
			return handlers.ToggleMessageUsefulness(messageId, c, app)
		})

		// populate threads list in sidebar
		e.Router.GET("/threads", func(c echo.Context) error {
			return handlers.GetThreadList("creation", c, app)
		})

		// click on thread to load messages
		e.Router.GET("/thread/:id", func(c echo.Context) error {
			threadId := c.PathParam("id")
			return handlers.GetThread(threadId, c, app)
		})

		// open thread title editor
		e.Router.GET("/thread/title/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			return handlers.EditThreadTitle(id, c, app)
		})

		// update thread title
		e.Router.PUT("/thread/title/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			data := apis.RequestInfo(c).Data
			title := data["title"].(string)
			return handlers.SaveThreadTitle(id, title, c, app)
		})

		// sort threads list
		e.Router.GET("/sort/:method", func(c echo.Context) error {
			method := c.PathParam("method")
			return handlers.GetThreadList(method, c, app)
		})

		// create new thread
		e.Router.POST("/thread/create", func(c echo.Context) error {
			return handlers.CreateThread(c, app)
		})

		// delete thread
		e.Router.DELETE("/thread/:threadId", func(c echo.Context) error {
			id := c.PathParam("threadId")
			return handlers.DeleteThread(id, c, app)
		})
		
		// load APIs for chat window dropdown
		e.Router.GET("/apis", func(c echo.Context) error {
			return handlers.LoadApis(c, app)
		})

		// attempt to load list of models an API provides
		e.Router.POST("/apis/models", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			id := data["api"].(string)
			return handlers.LoadApiModels(id, c, app)
		})

		// select an API from chat window dropdown
		e.Router.PUT("/api/select", func(c echo.Context) error {
			//model := c.QueryParam("model")
			data := apis.RequestInfo(c).Data
			id := data["api"].(string)
			return handlers.SelectApi(id, &selectedModel, c, app)
		})

		// select a model from chat window dropdown
		e.Router.POST("/apis/model", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			name := data["api-model-name"].(string)
			return handlers.SelectModel(name, c, app)
		})

		// open API editor in the sidebar
		e.Router.GET("/apis/open", func(c echo.Context) error {
			return handlers.OpenApiEditor(c, app)
		})

		// create new API in the sidebar
		e.Router.POST("/apis/create", func(c echo.Context) error {
			return handlers.CreateApi(c, app)
		})

		// delete an API in the sidebar
		e.Router.DELETE("/apis/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			return handlers.DeleteApi(id, c, app)
		})

		// update existing API in the sidebar
		e.Router.PATCH("/apis/update/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			data := apis.RequestInfo(c).Data
			return handlers.UpdateApi(id, data, c, app)
		})

		// open editor to create new tag
		e.Router.GET("/thread/tag/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			return handlers.CreateTag(id, c, app)
		})

		// create new tag and add to current thread
		e.Router.POST("/thread/tag/:id", func(c echo.Context) error {
			id := c.PathParam("id")
			data := apis.RequestInfo(c).Data
			return handlers.SaveTag(id, data, c, app)
		})

		// open editor to update tag, remove from thread, or delete entirely
		e.Router.GET("/tag/:tagId/thread/:threadId", func(c echo.Context) error {
			tagId := c.PathParam("tagId")
			threadId := c.PathParam("threadId")
			return handlers.OpenTagModifier(tagId, threadId, c, app)
		})

		// update tag and reload threads
		e.Router.POST("/tag/update/:tagId", func(c echo.Context) error {
			tagId := c.PathParam("tagId")
			data := apis.RequestInfo(c).Data
			return handlers.UpdateTag(tagId, data, c, app)
		})

		// delete tag and reload threads
		e.Router.DELETE("/tag/:tagId", func(c echo.Context) error {
			tagId := c.PathParam("tagId")
			return handlers.DeleteTag(tagId, c, app)
		})

		// remove tag from thread
		e.Router.DELETE("/thread/:threadId/tag/:tagId", func(c echo.Context) error {
			threadId := c.PathParam("threadId")
			tagId := c.PathParam("tagId")
			return handlers.RemoveTagFromThread(threadId, tagId, c, app)
		})

		// add existing tag to thread
		e.Router.POST("/thread/:threadId/tag/:tagId", func(c echo.Context) error {
			threadId := c.PathParam("threadId")
			tagId := c.PathParam("tagId")
			return handlers.AddExistingTagToThread(threadId, tagId, c, app)
		})

		// open config
		e.Router.GET("/config", func(c echo.Context) error {
			return handlers.OpenConfig(c, app)
		})

		// fetch model stats
		e.Router.GET("/stats", func(c echo.Context) error {
			return handlers.GetModelStats(c, app)
		})
		// update config
		e.Router.PUT("/config", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			return handlers.SaveConfig(data, c, app)
		})

		e.Router.GET("/config/done", func(c echo.Context) error {
			return handlers.GetThreadList("creation", c, app)
		})

		// open search in sidebar
		e.Router.GET("/search", func(c echo.Context) error {
			return handlers.OpenSearch(c, app)
		})

		// search for threads in sidebar
		e.Router.POST("/search", func(c echo.Context) error {
			data := apis.RequestInfo(c).Data
			return handlers.Search(data, c, app)
		})

		// open websocket connection for chat:
		e.Router.GET("/ws", func(c echo.Context) error {
			return handlers.OpenChatSocket(&selectedModel, c, app)
		})

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
