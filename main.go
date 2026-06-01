package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/ys-ll/uniterm/backend/log"
)

var Version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Capture top-level panics
	defer func() {
		if r := recover(); r != nil {
			_ = log.Init()
			log.Writef("FATAL PANIC: %v\n%s", r, string(debug.Stack()))
			log.Close()
			os.Exit(1)
		}
	}()

	if err := log.Init(); err != nil {
		println("Failed to init log:", err.Error())
	}
	defer log.Close()

	webviewDataPath := filepath.Join(os.TempDir(), fmt.Sprintf("uniTerm-webview2-%d", os.Getpid()))
	os.MkdirAll(webviewDataPath, 0700)

	app := NewApp(webviewDataPath)

	err := wails.Run(&options.App{
		Title:  "uniTerm",
		Width:  1200,
		Height:    800,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Windows: &windows.Options{
			WebviewUserDataPath: webviewDataPath,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		fmt.Println("Error:", err.Error())
		log.Writef("Wails run error: %v", err)
	}
}
