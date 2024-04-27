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

    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/apis"
    "github.com/pocketbase/pocketbase/core"
)

var (
    upgrader = websocket.Upgrader{}
)

func main() {
    app := pocketbase.New()

    // serve static files from public dir
    app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
        e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))

        // begin endpoints
        e.Router.GET("/threads", func(c echo.Context) error {
            c.Response().Writer.WriteHeader(200)

            entryParams := templates.ThreadListEntryParams{
                ThreadTitle: "Test title",
                Model: "Llama 3",
                StartTime: 111111111,
                LastMessageTime: 444444444,
                Tags: []string{"TagA", "TagB"},
                LastMessage: "This is the last message in the chat and...",
            }
            threadListEntry := templates.ThreadListEntry(entryParams)

            err := threadListEntry.Render(context.Background(), c.Response().Writer)
            if err != nil {
                return c.String(http.StatusInternalServerError, "failed to render thread list repsonse")
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

                // write
                chatParams := templates.ChatMessageParams{
                    Message: htmxMsg.Msg,
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
