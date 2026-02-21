/*
 * LAN Dock - Your network's digital loading bay.
 * Copyright (C) 2026 Mano K
 * * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	_ "embed"
	"image/color"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ncruces/zenity"

	// IMPORTANT: Change this to your actual module name found in go.mod
	"landock/assets"
	"landock/backend"
)


type AppState struct {
	isRunning bool
	sharePath string
	engine    *backend.Engine // Reference to our headless backend
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
	myWindow.Resize(fyne.NewSize(380, 650))
	myWindow.SetFixedSize(true)

	// UI Components setup
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

	lblPath := widget.NewLabel("No folder selected")
	lblPath.Wrapping = fyne.TextTruncate

	// QR Code Section
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

	// Links & Status
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

	// UI State Updater
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

	// Folder Selection
	btnChoose = widget.NewButton("Choose Folder", func() {
		dir, err := zenity.SelectFile(
			zenity.Title("Select Folder to Share"),
			zenity.Directory(),
		)
		if err == nil && dir != "" {
			state.sharePath = dir
			lblPath.SetText(dir)
		}
	})

	// Server Start/Stop Logic bridging to Backend Engine
	btnStart = widget.NewButton("Start Server", func() {
		if state.isRunning {
			state.engine.Stop()
			updateUI(false)
		} else {
			if state.sharePath == "" {
				zenity.Error("Please select a folder first.")
				return
			}

			// Call the backend Engine!
			fullURL, qrDataBytes, err := state.engine.Start(state.sharePath)
			if err != nil {
				zenity.Error("Server Error: " + err.Error())
				return
			}

			// Update UI with data from backend
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

	// Assemble Window
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

	// Intercept window close to ensure server stops cleanly
	myWindow.SetCloseIntercept(func() {
		if state.isRunning {
			state.engine.Stop()
		}
		myWindow.Close()
	})

	myWindow.SetContent(container.NewPadded(container.NewPadded(content)))
	myWindow.ShowAndRun()
}

type customTheme struct { fyne.Theme }
func (m *customTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Bold { return fyne.NewStaticResource("SpaceMono-Bold.ttf", assets.FontBold) }
	return fyne.NewStaticResource("SpaceMono-Regular.ttf", assets.FontRegular)
}