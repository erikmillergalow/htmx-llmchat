package main

import (
    "fmt"
    "log"
    "os"
    "context"
    "net/http"
    "bytes"
    "encoding/json"
    "errors"
    "io"

    "github.com/erikmillergalow/htmx-llmchat/templates"

    "github.com/labstack/echo/v5"
    "github.com/gorilla/websocket"   
    openai "github.com/sashabaranov/go-openai"

    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/apis"
    "github.com/pocketbase/pocketbase/core"
    "github.com/pocketbase/pocketbase/forms"
    "github.com/pocketbase/pocketbase/models"

)

var (
    upgrader = websocket.Upgrader{}
)

func main() {
    app := pocketbase.New()

    // this should be initialized when selecting a model
    // may be initializing API like this, may be spinning up local model
    chatgptClient := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

    // serve static files from public dir
    app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
        e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))

        // begin endpoints
        e.Router.GET("/threads", func(c echo.Context) error {
            threads := []struct{
                Id string `db:"id" json:"id"`
                Title string `db:"thread_title" json:"thread_title"`
                LastMessage string `db:"last_message" json:"last_message"`
            }{}

            app.Dao().DB().Select("*").From("chat_meta").All(&threads)

            fmt.Println(threads)

            var allEntryParams []templates.ThreadListEntryParams
            for _, thread := range threads {
                entryParams := templates.ThreadListEntryParams{
                    Id: thread.Id,
                    ThreadTitle: thread.Title,
                    LastMessage: thread.LastMessage,
                }
                allEntryParams = append(allEntryParams, entryParams)
                    
            }

            c.Response().Writer.WriteHeader(200)

            threadListEntry := templates.ThreadListEntry(allEntryParams)

            err := threadListEntry.Render(context.Background(), c.Response().Writer)
            if err != nil {
                return c.String(http.StatusInternalServerError, "failed to render thread list repsonse")
            }

            return nil               
        })

        e.Router.POST("/new-thread", func(c echo.Context) error {
            threadsCollection, err := app.Dao().FindCollectionByNameOrId("chat_meta")
            if err != nil {
                fmt.Println("error reading threads DB")
            }

            newThreadRecord := models.NewRecord(threadsCollection)

            form := forms.NewRecordUpsert(app, newThreadRecord)

            fmt.Println(form)

            if err := form.Submit(); err != nil {
                fmt.Println("error creating new thread")
            }

            fmt.Println(form)

            return nil            
        })

        // websocket connection:
        e.Router.GET("/ws", func(c echo.Context) error {
            ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
            if err != nil {
                fmt.Println("websocket upgrade failed")
                fmt.Println(err)
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
                }
                fmt.Printf("%s\n", msg)

                type HTMXSocketMsg struct {
                    Msg string `json:"new-message"`
                    Headers map[string]string `json:"HEADERS"`
                }
                var htmxMsg HTMXSocketMsg
                err = json.Unmarshal(msg, &htmxMsg)
                if err != nil {
                    fmt.Println("error parsing message")
                    fmt.Println(err)
                }
                fmt.Println(htmxMsg)

                // create message and response upserts
                requestRecord := models.NewRecord(chatCollection)
                form := forms.NewRecordUpsert(app, requestRecord)
                
                form.LoadData(map[string]any{
                    "message": htmxMsg.Msg,
                    "sender": "human",
                })

                if err := form.Submit(); err != nil {
                    fmt.Printf("Failed to submit user message to chat DB: %v\n")
                }
                // initialize new record for model, load data nd submit at end
                modelRecord := models.NewRecord(chatCollection)
                modelForm := forms.NewRecordUpsert(app, modelRecord)

                modelForm.LoadData(map[string]any{
                    "message": "",
                    "sender": "model",
                })

                if err := modelForm.Submit(); err != nil {
                    fmt.Printf("Failed to initialize model message in chat DB: %v\n")
                }

                // send the initial response skeleton
                chatParams := templates.ChatMessageParams{
                    Id: modelRecord.Id,
                    UserMessage: htmxMsg.Msg,
                }
                chatComponent := templates.ChatMessage(chatParams)

                // create ws write function to handle these
                var htmlBuf bytes.Buffer
                err = chatComponent.Render(context.Background(), &htmlBuf)
                if err != nil {
                    fmt.Println("templ render error")
                    fmt.Println(err)
                }
                htmlStr := htmlBuf.String()
                err = ws.WriteMessage(websocket.TextMessage, []byte(htmlStr))
                if err != nil {
                    fmt.Println("socket write failure")
                    fmt.Println(err)
                }

                // begin OpenAI specific request

                req := openai.ChatCompletionRequest{
                    Model: openai.GPT4,
                    MaxTokens: 20,
                    Messages: []openai.ChatCompletionMessage{
                        {
                            Role: openai.ChatMessageRoleUser,
                            Content: htmxMsg.Msg,
                        },
                    },
                    Stream: true,
                }
                stream, err := chatgptClient.CreateChatCompletionStream(context.Background(), req)
                if err != nil {
                    fmt.Printf("ChatCompletionStream error: %v\n", err)
                }
                defer stream.Close()

                fmt.Printf("Stream response: ")
               
                
                fullResponse := "" 
                for {
                    response, err := stream.Recv()
                    if errors.Is(err, io.EOF) {
                        // save response to DB here?
                        fmt.Println("\nStream finished")
                        break;
                    }

                    if err != nil {
                        fmt.Printf("\nStream error: %v\n", err)
                    }
                    fmt.Printf(response.Choices[0].Delta.Content)

                    fullResponse += response.Choices[0].Delta.Content
                    
                    responseChunkComponent := templates.ChatStreamChunk(modelRecord.Id, response.Choices[0].Delta.Content)
                    
                    var htmlBuf bytes.Buffer

                    err = responseChunkComponent.Render(context.Background(), &htmlBuf)
                    if err != nil {
                        fmt.Println("templ stream chunk render error")
                        fmt.Println(err)
                    }
                    htmlStr := htmlBuf.String()
                    fmt.Println(htmlStr)
                    err = ws.WriteMessage(websocket.TextMessage, []byte(htmlStr))
                    if err != nil {
                        fmt.Println("socket write failure")
                        fmt.Println(err)
                    }
                }

                // *** move above into own package

                // record model message in DB
                modelForm.LoadData(map[string]any{
                    "message": fullResponse,
                    "sender": "model",
                })

                if err := modelForm.Submit(); err != nil {
                    fmt.Printf("Failed to submit model message to chat DB: %v\n")
                }

            }
        })

        return nil
    })

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
