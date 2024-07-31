package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	openai "github.com/sashabaranov/go-openai"

	_ "github.com/erikmillergalow/htmx-llmchat/migrations"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/types"
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

		e.Router.GET("/model", func(c echo.Context) error {
			fmt.Println("select model")

			model := c.QueryParam("model")
			fmt.Println(model)

			if model == "openai" {
				c.Response().Writer.WriteHeader(200)
				selectModelStatus := templates.SelectModelStatus("Now chatting with OpenAI")
				selectedModel = "openai"
				err := selectModelStatus.Render(context.Background(), c.Response().Writer)
				if err != nil {
					return c.String(http.StatusInternalServerError, "failed to render select model status")
				}
			} else if model == "groq" {
				c.Response().Writer.WriteHeader(200)
				selectModelStatus := templates.SelectModelStatus("Now chatting with Groq API")
				err := selectModelStatus.Render(context.Background(), c.Response().Writer)
				selectedModel = "groq"
				if err != nil {
					return c.String(http.StatusInternalServerError, "failed to render select model status")
				}

			} else {
				return c.String(http.StatusInternalServerError, "model not recognized")
			}

			return nil
		})

		e.Router.POST("/thread/create", func(c echo.Context) error {
			return CreateThread(c, app)
		})

		e.Router.GET("/thread/tag/:id", func(c echo.Context) error {
			fmt.Println("open tag editor")

			id := c.PathParam("id")

			c.Response().Writer.WriteHeader(200)
			tagEditor := templates.NewTagEditor(id)
			err := tagEditor.Render(context.Background(), c.Response().Writer)
			if err != nil {
				return c.String(http.StatusInternalServerError, "failed to render tag editor")
			}

			return nil
		})

		e.Router.POST("/thread/tag/:id", func(c echo.Context) error {
			fmt.Println("create tag")

			id := c.PathParam("id")

			data := apis.RequestInfo(c).Data
			fmt.Println(data)
			value := data["value"].(string)
			color := data["color"].(string)

			tagsCollection, err := app.Dao().FindCollectionByNameOrId("tags")
			if err != nil {
				return c.String(http.StatusInternalServerError, "error reading tags DB")
			}

			newTagRecord := models.NewRecord(tagsCollection)
			form := forms.NewRecordUpsert(app, newTagRecord)

			form.LoadData(map[string]any{
				"value": value,
				"color": color,
			})

			if err := form.Submit(); err != nil {
				return c.String(http.StatusInternalServerError, "failed to create new tag DB entry")
			}

			threadRecord, err := app.Dao().FindRecordById("chat_meta", id)
			if err != nil {
				return c.String(http.StatusInternalServerError, "failed to read thread DB")
			}

			threadRecord.Set("tags", append(threadRecord.GetStringSlice("tags"), newTagRecord.Id))
			if err = app.Dao().SaveRecord(threadRecord); err != nil {
				return c.String(http.StatusInternalServerError, "failed to add tag to thread")
			}

			tagParams := templates.TagParams{
				Id:       newTagRecord.Id,
				Value:    value,
				ThreadId: id,
				Color:    color,
			}

			c.Response().Writer.WriteHeader(200)
			newTag := templates.NewTag(tagParams)
			err = newTag.Render(context.Background(), c.Response().Writer)
			if err != nil {
				return c.String(http.StatusInternalServerError, "failed to render tag editor")
			}

			return nil
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

		e.Router.GET("/config", func(c echo.Context) error {
			fmt.Println("opening config")

			// this should be associated with user accounts for server style setup
			var settings []templates.SideBarMenuParams
			app.Dao().DB().
				Select("*").
				From("settings").
				All(&settings)

			fmt.Println(settings)

			c.Response().Writer.WriteHeader(200)
			loadedSettingsMenu := templates.SideBarMenu(settings[0])
			err := loadedSettingsMenu.Render(context.Background(), c.Response().Writer)
			if err != nil {
				return c.String(http.StatusInternalServerError, "failed to render loaded chat response")
			}

			return nil
		})

		e.Router.PUT("/config", func(c echo.Context) error {
			fmt.Println("saving config")

			// this should be associated with user accounts for server style setup
			settingsRecord, err := app.Dao().FindFirstRecordByData("settings", "type", "keys")
			if err != nil {
				return c.String(http.StatusInternalServerError, "failed to fetch keys record")
			}

			data := apis.RequestInfo(c).Data
			fmt.Printf("settings data: %v\n", data)
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
		})

		e.Router.GET("/config/done", func(c echo.Context) error {
			fmt.Println("closing config")

			// this is currently the same as the /threads endpoint
			fmt.Println("load all threads")
			var threads []templates.ThreadListEntryParams
			app.Dao().DB().Select("*").From("chat_meta").OrderBy("created DESC").All(&threads)

			var allTags [][]templates.TagParams

			// load thread tags
			for _, thread := range threads {
				var threadTags []templates.TagParams
				threadRecord, err := app.Dao().FindRecordById("chat_meta", thread.Id)
				if err != nil {
					return c.String(http.StatusInternalServerError, "failed to fetch thread record")
				}
				if errs := app.Dao().ExpandRecord(threadRecord, []string{"tags"}, nil); len(errs) > 0 {
					return c.String(http.StatusInternalServerError, "failed to expand thread tags")
				}
				for _, expandedTag := range threadRecord.ExpandedAll("tags") {
					fmt.Println(expandedTag)
					threadTags = append(threadTags, templates.TagParams{
						Value:    expandedTag.GetString("value"),
						ThreadId: expandedTag.GetString("value"),
						Color:    expandedTag.GetString("color"),
						Id:       expandedTag.Id,
					})
				}
				allTags = append(allTags, threadTags)
			}

			c.Response().Writer.WriteHeader(200)
			threadListEntry := templates.ThreadListEntries(threads, allTags)
			fmt.Println(threadListEntry)
			err := threadListEntry.Render(context.Background(), c.Response().Writer)
			if err != nil {
				return c.String(http.StatusInternalServerError, "failed to render thread list repsonse")
			}

			return nil

		})

		// websocket connection:
		e.Router.GET("/ws", func(c echo.Context) error {
			fmt.Println("websocket triggered")
			ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
			if err != nil {
				fmt.Println("websocket upgrade failed")
				fmt.Println(err)
				return err
			}
			defer ws.Close()

			chatCollection, err := app.Dao().FindCollectionByNameOrId("chat")
			if err != nil {
				return err
			}

			for {
				// read
				_, msg, err := ws.ReadMessage()
				if err != nil {
					fmt.Println("socket read failure")
					fmt.Println(err)
					return err
				}
				fmt.Printf("%s\n", msg)

				type HTMXSocketMsg struct {
					Headers  map[string]string `json:"HEADERS"`
					Msg      string            `json:"new-message"`
					ThreadId string            `json:"thread-id-chat"`
				}
				var htmxMsg HTMXSocketMsg
				err = json.Unmarshal(msg, &htmxMsg)
				if err != nil {
					fmt.Printf("error parsing message: %v\n", err)
					return err
				}
				fmt.Println(htmxMsg)

				if htmxMsg.Msg != "" {

					// create message and response upserts
					requestRecord := models.NewRecord(chatCollection)
					form := forms.NewRecordUpsert(app, requestRecord)

					form.LoadData(map[string]any{
						"thread_id": htmxMsg.ThreadId,
						"message":   htmxMsg.Msg,
						"sender":    "human",
						"model":     selectedModel,
					})

					if err := form.Submit(); err != nil {
						fmt.Printf("Failed to submit user message to chat DB: %v\n", err)
						return err
					}
					// initialize new record for model, load data nd submit at end
					modelRecord := models.NewRecord(chatCollection)
					modelForm := forms.NewRecordUpsert(app, modelRecord)

					modelForm.LoadData(map[string]any{
						"thread_id": htmxMsg.ThreadId,
						"message":   "",
						"sender":    "model",
					})

					if err := modelForm.Submit(); err != nil {
						fmt.Printf("Failed to initialize model message in chat DB: %v\n", err)
						return err
					}

					// send the initial response skeleton
					chatParams := templates.ChatMessageParams{
						Id:          modelRecord.Id,
						UserMessage: htmxMsg.Msg,
						Model:       selectedModel,
					}
					chatComponent := templates.ChatMessage(chatParams)

					// create ws write function to handle these
					var htmlBuf bytes.Buffer
					err = chatComponent.Render(context.Background(), &htmlBuf)
					if err != nil {
						fmt.Println("templ render error")
						fmt.Println(err)
						return err
					}
					htmlStr := htmlBuf.String()
					err = ws.WriteMessage(websocket.TextMessage, []byte(htmlStr))
					if err != nil {
						fmt.Println("socket write failure")
						fmt.Println(err)
						return err
					}

					// begin OpenAI specific request
					// get all chats from thread, create ChatCompletionMessage
					var messages []templates.LoadedMessageParams
					app.Dao().DB().
						Select("*").
						From("chat").
						Where(dbx.NewExp("thread_id = {:id}", dbx.Params{"id": htmxMsg.ThreadId})).
						All(&messages)

					// TODO: utilize ChatMessageRoleSystem for system prompt (ex: "You are a helpful assistant")
					var chatHistory []openai.ChatCompletionMessage
					for _, message := range messages {
						if message.Sender == "human" {
							chatHistory = append(chatHistory, openai.ChatCompletionMessage{
								Role:    openai.ChatMessageRoleUser,
								Content: message.Message,
							})
						} else {
							chatHistory = append(chatHistory, openai.ChatCompletionMessage{
								Role:    openai.ChatMessageRoleAssistant,
								Content: message.Message,
							})
						}
					}

					var settings []templates.SideBarMenuParams
					app.Dao().DB().
						Select("*").
						From("settings").
						All(&settings)

					model := openai.GPT4
					if selectedModel == "groq" {
						model = "llama3-70b-8192"
						config := openai.DefaultConfig(settings[0].GroqKey)
						config.BaseURL = "https://api.groq.com/openai/v1"
						chatgptClient = openai.NewClientWithConfig(config)
					} else {
						chatgptClient = openai.NewClient(settings[0].OpenAIKey)
					}

					req := openai.ChatCompletionRequest{
						Model: model,
						// MaxTokens: 20,
						Messages: chatHistory,
						Stream:   true,
					}
					stream, err := chatgptClient.CreateChatCompletionStream(context.Background(), req)
					if err != nil {
						fmt.Printf("ChatCompletionStream error: %v\n", err)
						return err
					}
					defer stream.Close()

					fmt.Printf("Stream response: ")

					fullResponse := ""
					for {
						response, err := stream.Recv()
						if errors.Is(err, io.EOF) {
							fmt.Println("\nStream finished")
							break
						}

						if err != nil {
							fmt.Printf("\nStream error: %v\n", err)
							return err
						}

						fullResponse += response.Choices[0].Delta.Content

						responseChunkComponent := templates.ChatStreamChunk(modelRecord.Id, response.Choices[0].Delta.Content)

						var htmlBuf bytes.Buffer

						err = responseChunkComponent.Render(context.Background(), &htmlBuf)
						if err != nil {
							fmt.Println("templ stream chunk render error")
							fmt.Println(err)
							return err
						}
						htmlStr := htmlBuf.String()
						fmt.Println(htmlStr)
						err = ws.WriteMessage(websocket.TextMessage, []byte(htmlStr))
						if err != nil {
							fmt.Println("socket write failure")
							fmt.Println(err)
							return err
						}
					}

					// *** move above into own package

					// record model message in DB
					modelForm.LoadData(map[string]any{
						"thread_id": htmxMsg.ThreadId,
						"message":   fullResponse,
						"sender":    "model",
						"model":     selectedModel,
					})

					if err := modelForm.Submit(); err != nil {
						fmt.Printf("Failed to submit model message to chat DB: %v\n", err)
						return err
					}

					threadRecord, err := app.Dao().FindRecordById("chat_meta", htmxMsg.ThreadId)
					if err != nil {
						fmt.Printf("Error reading thread metadata: %v\n", err)
						return err
					}

					lastMessageTime := types.NowDateTime()
					responseChunkComponent := templates.LastMessageTimestamp(htmxMsg.ThreadId, lastMessageTime)

					// sending HTML along websocket needs to be pulled to it's own function
					var lastMessageTimeBuf bytes.Buffer

					err = responseChunkComponent.Render(context.Background(), &lastMessageTimeBuf)
					if err != nil {
						fmt.Println("templ stream chunk render error")
						fmt.Println(err)
						return err
					}
					lastMessageTimeStr := lastMessageTimeBuf.String()
					fmt.Println(lastMessageTimeStr)
					err = ws.WriteMessage(websocket.TextMessage, []byte(lastMessageTimeStr))
					if err != nil {
						fmt.Println("socket write failure")
						fmt.Println(err)
						return err
					}

					threadRecord.Set("last_message_timestamp", types.NowDateTime())
					threadRecord.Set("last_message", fullResponse[0:10])
					if err := app.Dao().SaveRecord(threadRecord); err != nil {
						fmt.Printf("Error updating thread metadata: %v\n", err)
						return err
					}
				}
			}
		})

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
