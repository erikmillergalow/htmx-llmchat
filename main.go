package main

import (
    "log"
    "os"
    "context"
    "net/http"

    "github.com/erikmillergalow/htmx-llmchat/templates"

    "github.com/labstack/echo/v5"
    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/apis"
    "github.com/pocketbase/pocketbase/core"
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

        return nil
    })

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
