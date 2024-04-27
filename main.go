package main

import (
    "fmt"
    "log"
    "os"
    "context"
    "net/http"
    "bytes"
    "encoding/json"

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

                // query model TODO: generalize this to all model types
                resp, err := chatgptClient.CreateChatCompletion(
                    context.Background(),
                    openai.ChatCompletionRequest{
                        Model: openai.GPT4,
                        Messages: []openai.ChatCompletionMessage{
                            {
                                Role: openai.ChatMessageRoleUser,
                                Content: htmxMsg.Msg,
                            },
                        },
                    },
                )

                if err != nil {
                    fmt.Printf("ChatCompletion error %v\n", err)
                }
                // *** move above into own package

                modelResponse := resp.Choices[0].Message.Content
                fmt.Println(modelResponse)

                // write
                chatParams := templates.ChatMessageParams{
                    ModelMessage: modelResponse,
                    UserMessage: htmxMsg.Msg,
                }
                chatComponent := templates.ChatMessage(chatParams)
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
            }
        })

        return nil
    })

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
