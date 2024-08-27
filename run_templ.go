package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
)

func main() {
    // gopath := os.Getenv("GOPATH")
    // if gopath == "" {
    //     fmt.Println("GOPATH is not set")
    //     os.Exit(1)
    // }

    gopath := "/home/runner/go"
    if os.Getenv("GOOS") == "windows" {
        gopath = "C:\\Users\\runneradmin\\go"
    } else if os.Getenv("GOOS") == "darwin" {
        gopath = "/Users/runner/go"
    }

    templPath := filepath.Join(gopath, "bin", "templ")
    if os.Getenv("GOOS") == "windows" {
        templPath += ".exe"
    }

    cmd := exec.Command(templPath, "generate")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    err := cmd.Run()
    if err != nil {
        fmt.Printf("Error running templ: %v\n", err)
        os.Exit(1)
    }
}
