package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/erikmillergalow/htmx-llmchat/templates"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	openai "github.com/sashabaranov/go-openai"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/types"
)

type HTMXSocketMsg struct {
	Headers  map[string]string `json:"HEADERS"`
	Msg      string            `json:"new-message"`
	ThreadId string            `json:"thread-id-chat"`
}

func OpenChatSocket(selectedModel *string, c echo.Context, app *pocketbase.PocketBase) error {
	fmt.Println("websocket triggered")
	
	var upgrader = websocket.Upgrader{
       CheckOrigin: func(r *http.Request) bool {
           // Allow all connections by returning true.
           // For better security, you can specify conditions here.
           return true

           // Example for specific origin:
           // return r.Header.Get("Origin") == "tauri://localhost"
       },
   }
	
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

		var htmxMsg HTMXSocketMsg
		err = json.Unmarshal(msg, &htmxMsg)
		if err != nil {
			fmt.Printf("error parsing message: %v\n", err)
			return err
		}
		fmt.Println(htmxMsg)

		if htmxMsg.Msg != "" {

			// fetch selected model config
			userRecord, err := app.Dao().FindFirstRecordByData("users", "username", "default")
			if err != nil {
				return err
			}

			selectedModelRecord, err := app.Dao().FindRecordById("apis", userRecord.GetString("selected_api"))
			if err != nil {
				return err
			}

			chatModelName := selectedModelRecord.GetString("name")
			if userRecord.GetString("selected_model_name") != "" {
				chatModelName = chatModelName + "-" + userRecord.GetString("selected_model_name")
			}

			// create message and response upserts
			requestRecord := models.NewRecord(chatCollection)
			form := forms.NewRecordUpsert(app, requestRecord)

			// store message from human
			form.LoadData(map[string]any{
				"thread_id": htmxMsg.ThreadId,
				"message":   htmxMsg.Msg,
				"sender":    "human",
				"model":     chatModelName,
			})
			if err := form.Submit(); err != nil {
				fmt.Printf("Failed to submit user message to chat DB: %v\n", err)
				return err
			}

			// initialize new record for model, load data and submit at end
			modelMessageRecord := models.NewRecord(chatCollection)
			modelForm := forms.NewRecordUpsert(app, modelMessageRecord)
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
				Id:          modelMessageRecord.Id,
				UserMessage: htmxMsg.Msg,
				Model:       chatModelName,
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

			// get all chats from thread, create ChatCompletionMessage so model has context
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

			config := openai.DefaultConfig(selectedModelRecord.GetString("api_key"))
			config.BaseURL = selectedModelRecord.GetString("url")
			chatgptClient := openai.NewClientWithConfig(config)

			req := openai.ChatCompletionRequest{
				Model: userRecord.GetString("selected_model_name"),
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

				responseChunkComponent := templates.ChatStreamChunk(modelMessageRecord.Id, response.Choices[0].Delta.Content)

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

			// record model message in DB
			modelForm.LoadData(map[string]any{
				"thread_id": htmxMsg.ThreadId,
				"message":   fullResponse,
				"sender":    "model",
				"model":     chatModelName,
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
}
