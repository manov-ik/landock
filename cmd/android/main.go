package main

import (
	"errors"
	"image/color"
	"net/url"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"landock/assets"
	"landock/backend"
)

type AppState struct {
	isRunning bool
	sharePath string
	engine    *backend.Engine
}

func getShareDir() string {
	candidates := []string{
		"/storage/emulated/0/Download",
		"/storage/emulated/0/Downloads",
		"/storage/emulated/0/Documents",
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/data/data/com.landock.app"
	}
	fallback := filepath.Join(home, "files", "landock-share")
	os.MkdirAll(fallback, 0755)
	return fallback
}

func main() {
	state := &AppState{
		engine: backend.NewEngine(),
	}

	myApp := app.NewWithID("com.landock.app")
	if len(assets.IconData) > 0 {
		myApp.SetIcon(fyne.NewStaticResource("icon.png", assets.IconData))
	}
	myApp.Settings().SetTheme(&customTheme{Theme: theme.DarkTheme()})

	myWindow := myApp.NewWindow("LAN Dock")

	logoRes := fyne.NewStaticResource("logo.png", assets.LogoData)
	logoImg := canvas.NewImageFromResource(logoRes)
	logoImg.FillMode = canvas.ImageFillContain
	logoImg.SetMinSize(fyne.NewSize(64, 64))

	title := canvas.NewText("LAN Dock", color.White)
	title.TextSize = 32
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	lblHeader := canvas.NewText("Shared Folder", color.RGBA{150, 150, 150, 255})
	lblHeader.TextSize = 12
	lblHeader.Alignment = fyne.TextAlignLeading

	lblPath := widget.NewLabel("No folder selected")
	lblPath.Wrapping = fyne.TextTruncate

	qrBoxBg := canvas.NewRectangle(color.White)
	qrBoxBg.CornerRadius = 18
	qrBoxBg.SetMinSize(fyne.NewSize(240, 240))

	qrBoxText := canvas.NewText("QR", color.Black)
	qrBoxText.TextSize = 18
	qrBoxText.TextStyle = fyne.TextStyle{Bold: true}
	qrPlaceholder := container.NewStack(qrBoxBg, container.NewCenter(qrBoxText))

	qrImg := canvas.NewImageFromResource(nil)
	qrImg.FillMode = canvas.ImageFillContain
	qrImg.SetMinSize(fyne.NewSize(240, 240))
	qrImgContainer := container.NewStack(qrBoxBg, container.NewPadded(qrImg))
	qrImgContainer.Hide()

	qrArea := container.NewStack(qrPlaceholder, qrImgContainer)

	linkURL, _ := url.Parse("http://localhost")
	hyperlink := widget.NewHyperlink("", linkURL)
	hyperlink.Alignment = fyne.TextAlignCenter
	hyperlink.Hide()

	statusText := canvas.NewText("Stopped", color.RGBA{255, 59, 48, 255})
	statusText.TextSize = 16
	statusText.TextStyle = fyne.TextStyle{Bold: true}
	statusText.Alignment = fyne.TextAlignCenter

	var btnStart *widget.Button
	var btnChoose *widget.Button

	updateUI := func(running bool) {
		state.isRunning = running
		if running {
			statusText.Text = "Running"
			statusText.Color = color.RGBA{52, 199, 89, 255}
			btnStart.SetText("Stop Server")
			btnStart.Importance = widget.DangerImportance
			btnChoose.Disable()
			qrPlaceholder.Hide()
			qrImgContainer.Show()
			hyperlink.Show()
		} else {
			statusText.Text = "Stopped"
			statusText.Color = color.RGBA{255, 59, 48, 255}
			btnStart.SetText("Start Server")
			btnStart.Importance = widget.HighImportance
			btnChoose.Enable()
			qrImgContainer.Hide()
			qrPlaceholder.Show()
			hyperlink.Hide()
		}
		statusText.Refresh()
	}

	btnChoose = widget.NewButton("Use Downloads Folder", func() {
		p := getShareDir()
		state.sharePath = p
		lblPath.SetText(p)
	})

	btnStart = widget.NewButton("Start Server", func() {
		if state.isRunning {
			state.engine.Stop()
			updateUI(false)
		} else {
			if state.sharePath == "" {
				dialog.ShowError(errors.New("Please select a folder first"), myWindow)
				return
			}
			fullURL, qrDataBytes, err := state.engine.Start(state.sharePath)
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			res := fyne.NewStaticResource("qr.png", qrDataBytes)
			qrImg.Resource = res
			qrImg.Refresh()
			hyperlink.SetText(fullURL)
			u, _ := url.Parse(fullURL)
			hyperlink.URL = u
			hyperlink.Refresh()
			updateUI(true)
		}
	})
	btnStart.Importance = widget.HighImportance

	content := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(container.NewHBox(logoImg, title)),
		layout.NewSpacer(),
		container.NewVBox(lblHeader, container.NewPadded(lblPath), btnChoose),
		layout.NewSpacer(),
		container.NewCenter(qrArea),
		layout.NewSpacer(),
		container.NewVBox(hyperlink),
		layout.NewSpacer(),
		container.NewVBox(statusText),
		layout.NewSpacer(),
		btnStart,
	)

	myWindow.SetCloseIntercept(func() {
		if state.isRunning {
			state.engine.Stop()
		}
		myWindow.Close()
	})

	myWindow.SetContent(container.NewPadded(container.NewPadded(content)))
	myWindow.ShowAndRun()
}

type customTheme struct{ fyne.Theme }

func (m *customTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Bold {
		return fyne.NewStaticResource("SpaceMono-Bold.ttf", assets.FontBold)
	}
	return fyne.NewStaticResource("SpaceMono-Regular.ttf", assets.FontRegular)
}