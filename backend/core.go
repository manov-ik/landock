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

package backend

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
)

//go:embed frontend/*
var embeddedFiles embed.FS

const Port = ":5455"

func init() {
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".ico", "image/x-icon")
}

// Engine holds the state of our backend server
type Engine struct {
	srv        *http.Server
	cancelFunc context.CancelFunc
	QRData     []byte
	FullURL    string
}

// NewEngine creates a new backend instance
func NewEngine() *Engine {
	return &Engine{}
}

// Start launches the Gin server sharing the specified directory.
// It returns the full local URL and the generated QR code bytes.
func (e *Engine) Start(dir string) (string, []byte, error) {
	ip := getLocalIP()
	e.FullURL = fmt.Sprintf("http://%s%s", ip, Port)

	// Generate QR Code bytes
	var err error
	e.QRData, err = qrcode.Encode(e.FullURL, qrcode.Medium, 256)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate QR: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// API Routes
	r.POST("/upload", func(c *gin.Context) {
		form, _ := c.MultipartForm()
		files := form.File["files"]
		count := 0
		for _, file := range files {
			filename := filepath.Base(file.Filename)
			if filename == "." || filename == ".." {
				continue
			}
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
		for _, entry := range entries {
			if !entry.IsDir() {
				info, _ := entry.Info()
				fileList = append(fileList, gin.H{"name": entry.Name(), "size": info.Size()})
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
		if len(e.QRData) > 0 {
			c.Data(200, "image/png", e.QRData)
		} else {
			c.Status(404)
		}
	})

	// Serve the embedded React/Vite web frontend
	subFS, _ := fs.Sub(embeddedFiles, "frontend")
	fileServer := http.FileServer(http.FS(subFS))
	r.NoRoute(func(c *gin.Context) { gin.WrapH(fileServer)(c) })

	e.srv = &http.Server{Addr: "0.0.0.0" + Port, Handler: r}

	var ctx context.Context
	ctx, e.cancelFunc = context.WithCancel(context.Background())
	_ = ctx

	// Run server in a goroutine so it doesn't block the UI thread
	go func() {
		if err := e.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("Server Error:", err)
		}
	}()

	return e.FullURL, e.QRData, nil
}

// Stop safely shuts down the server
func (e *Engine) Stop() {
	if e.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		e.srv.Shutdown(ctx)
	}
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
}

// Helper function to get local IP
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}