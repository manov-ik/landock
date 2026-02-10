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
	"context"
	"embed"
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/gin-gonic/gin"
	"github.com/ncruces/zenity"
	"github.com/skip2/go-qrcode"
)

//go:embed frontend/*
var embeddedFiles embed.FS

//go:embed icon1024.png
var iconData []byte

//go:embed logo-a.png
var logoData []byte

//go:embed SpaceMono-Regular.ttf
var fontRegular []byte

//go:embed SpaceMono-Bold.ttf
var fontBold []byte

const (
	Port = ":5455"
)

func init() {
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".ico", "image/x-icon")
}

type ServerState struct {
	srv        *http.Server
	isRunning  bool
	sharePath  string
	qrData     []byte
	cancelFunc context.CancelFunc
}

var state = &ServerState{}

func main() {
	myApp := app.NewWithID("com.landock.app")
	if len(iconData) > 0 {
		myApp.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	}

	myApp.Settings().SetTheme(&customTheme{Theme: theme.DarkTheme()})

	myWindow := myApp.NewWindow("LAN Dock")
	myWindow.Resize(fyne.NewSize(380, 650))
	myWindow.SetFixedSize(true)

	// Create the Logo Object
    logoRes := fyne.NewStaticResource("logo.png", logoData)
    logoImg := canvas.NewImageFromResource(logoRes)
    logoImg.FillMode = canvas.ImageFillContain
    logoImg.SetMinSize(fyne.NewSize(64, 64))

	// --- UI COMPONENTS ---

	title := canvas.NewText("LAN Dock", color.White)
	title.TextSize = 32
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	lblHeader := canvas.NewText("Shared Folder", color.RGBA{150, 150, 150, 255})
	lblHeader.TextSize = 12
	lblHeader.Alignment = fyne.TextAlignLeading

	lblPath := widget.NewLabel("No folder selected")
	lblPath.Wrapping = fyne.TextTruncate

	// --- QR CODE SECTION ---
	
	// Background: White Rounded Rectangle
	qrBoxBg := canvas.NewRectangle(color.White)
	qrBoxBg.CornerRadius = 18 // Visible Radius
	qrBoxBg.SetMinSize(fyne.NewSize(240, 240))

	// Placeholder Text
	qrBoxText := canvas.NewText("QR", color.Black)
	qrBoxText.TextSize = 18
	qrBoxText.TextStyle = fyne.TextStyle{Bold: true}
	qrPlaceholder := container.NewStack(qrBoxBg, container.NewCenter(qrBoxText))
	
	// Real QR Image
	qrImg := canvas.NewImageFromResource(nil)
	qrImg.FillMode = canvas.ImageFillContain
	qrImg.SetMinSize(fyne.NewSize(240, 240))
	
	// *** THE FIX: Wrap qrImg in NewPadded so it sits inside the rounded corners ***
	qrImgContainer := container.NewStack(qrBoxBg, container.NewPadded(qrImg))
	qrImgContainer.Hide()

	qrArea := container.NewStack(qrPlaceholder, qrImgContainer)

	// --- LINKS & STATUS ---
	
	linkURL, _ := url.Parse("http://localhost")
	hyperlink := widget.NewHyperlink("", linkURL)
	hyperlink.Alignment = fyne.TextAlignCenter
	hyperlink.Hide() 

	statusText := canvas.NewText("Stopped", color.RGBA{255, 59, 48, 255})
	statusText.TextSize = 16
	statusText.TextStyle = fyne.TextStyle{Bold: true}
	statusText.Alignment = fyne.TextAlignCenter

	// --- BUTTONS ---

	var btnStart *widget.Button
	var btnChoose *widget.Button

	updateUI := func(running bool) {
		state.isRunning = running
		if running {
			statusText.Text = "Running"
			statusText.Color = color.RGBA{52, 199, 89, 255}
			statusText.Refresh()
			btnStart.SetText("Stop Server")
			btnStart.Importance = widget.DangerImportance
			btnChoose.Disable()
			qrPlaceholder.Hide()
			qrImgContainer.Show()
			hyperlink.Show()
		} else {
			statusText.Text = "Stopped"
			statusText.Color = color.RGBA{255, 59, 48, 255}
			statusText.Refresh()
			btnStart.SetText("Start Server")
			btnStart.Importance = widget.HighImportance
			btnChoose.Enable()
			qrImgContainer.Hide()
			qrPlaceholder.Show()
			hyperlink.Hide()
		}
	}

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

	btnStart = widget.NewButton("Start Server", func() {
		if state.isRunning {
			stopServer()
			updateUI(false)
		} else {
			if state.sharePath == "" {
				zenity.Error("Please select a folder first.")
				return
			}

			ip := getLocalIP()
			fullURL := fmt.Sprintf("http://%s%s", ip, Port)

			var err error
			state.qrData, err = qrcode.Encode(fullURL, qrcode.Medium, 256)
			if err != nil {
				zenity.Error("QR Error: " + err.Error())
				return
			}

			res := fyne.NewStaticResource("qr.png", state.qrData)
			qrImg.Resource = res
			qrImg.Refresh()
			
			hyperlink.SetText(fullURL)
			u, _ := url.Parse(fullURL)
			hyperlink.URL = u
			hyperlink.Refresh()

			go startServer(state.sharePath, func(err error) {
				updateUI(false)
				fmt.Println("Server Error:", err)
			})

			updateUI(true)
		}
	})
	btnStart.Importance = widget.HighImportance

	// --- LAYOUT ---
	content := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(container.NewHBox(logoImg, title)),
			
		layout.NewSpacer(),
		
		container.NewVBox(
			lblHeader,
			container.NewPadded(lblPath),
			btnChoose,
		),
		
		layout.NewSpacer(),
		container.NewCenter(qrArea),
		layout.NewSpacer(),
		container.NewVBox(hyperlink),
		layout.NewSpacer(),
		container.NewVBox(statusText),
		layout.NewSpacer(),
		
		btnStart,
	)

	myWindow.SetContent(container.NewPadded(container.NewPadded(content)))
	myWindow.ShowAndRun()
}

// --- CUSTOM THEME ---

type customTheme struct {
	fyne.Theme
}

func (m *customTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Bold {
		return fyne.NewStaticResource("SpaceMono-Bold.ttf", fontBold)
	}
	return fyne.NewStaticResource("SpaceMono-Regular.ttf", fontRegular)
}


// --- SERVER LOGIC ---

func startServer(dir string, onError func(error)) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/upload", func(c *gin.Context) {
		form, _ := c.MultipartForm()
		files := form.File["files"]
		count := 0
		for _, file := range files {
			filename := filepath.Base(file.Filename)
			if filename == "." || filename == ".." { continue }
			c.SaveUploadedFile(file, filepath.Join(dir, filename))
			count++
		}
		c.JSON(200, gin.H{"count": count})
	})

	r.GET("/files", func(c *gin.Context) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			c.JSON(500, gin.H{"error": "Read error"})
			return
		}
		var fileList []gin.H
		for _, e := range entries {
			if !e.IsDir() {
				info, _ := e.Info()
				fileList = append(fileList, gin.H{"name": e.Name(), "size": info.Size()})
			}
		}
		c.JSON(200, fileList)
	})

	r.GET("/download", func(c *gin.Context) {
		name := c.Query("name")
		c.FileAttachment(filepath.Join(dir, filepath.Base(name)), filepath.Base(name))
	})

	r.DELETE("/delete", func(c *gin.Context) {
		os.Remove(filepath.Join(dir, filepath.Base(c.Query("name"))))
		c.JSON(200, gin.H{"status": "deleted"})
	})

	r.GET("/qr.png", func(c *gin.Context) {
		if len(state.qrData) > 0 {
			c.Data(200, "image/png", state.qrData)
		} else {
			c.Status(404)
		}
	})

	subFS, _ := fs.Sub(embeddedFiles, "frontend")
	fileServer := http.FileServer(http.FS(subFS))
	r.NoRoute(func(c *gin.Context) { gin.WrapH(fileServer)(c) })

	state.srv = &http.Server{Addr: "0.0.0.0" + Port, Handler: r}
	
	var ctx context.Context
	ctx, state.cancelFunc = context.WithCancel(context.Background())
	_ = ctx

	if err := state.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		onError(err)
	}
}

func stopServer() {
	if state.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		state.srv.Shutdown(ctx)
	}
	if state.cancelFunc != nil { state.cancelFunc() }
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil { return "127.0.0.1" }
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}