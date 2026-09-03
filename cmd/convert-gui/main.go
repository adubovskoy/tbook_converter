package main

import (
	"embed"
	"log"
	"os"

	"github.com/dimando/reader/converter/internal/cli"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

// cliSentinel switches the binary into converter-CLI mode:
// `tbook-converter convert book.epub -t ru …`. The GUI re-execs itself this
// way for every conversion, so one binary ships both the app and the CLI.
const cliSentinel = "convert"

func main() {
	if len(os.Args) > 1 && os.Args[1] == cliSentinel {
		os.Exit(cli.Main(os.Args[2:]))
	}

	svc, err := NewConvertService()
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "tBook Converter",
		Description: "Convert EPUB/FB2 books into tap-to-translate .tbook files",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.techlovers.treader.converter",
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "tBook Converter",
		Width:            1000,
		Height:           700,
		MinWidth:         720,
		MinHeight:        540,
		BackgroundColour: application.NewRGB(251, 251, 253),
		URL:              "/",
		EnableFileDrop:   true,
	})
	// Native drops deliver absolute paths only on the Go side; relay them to
	// the dropzone (elements marked data-file-drop-target).
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		if paths := e.Context().DroppedFiles(); len(paths) > 0 {
			app.Event.Emit(eventFilesDropped, paths)
		}
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
