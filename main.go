package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "Cover Downloader",
		Width:     900,
		Height:    720,
		MinWidth:  640,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
			// Serves /thumbs (print sheet thumbnails); used only for
			// requests that match no embedded asset.
			Handler: app.thumbs,
		},
		// Matches --bg in frontend/src/style.css (light theme).
		BackgroundColour: &options.RGBA{R: 245, G: 246, B: 248, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
