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
	"io"
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

// EventKind describes the type of transfer event.
type EventKind string

const (
	EventUpload   EventKind = "upload"
	EventDownload EventKind = "download"
	EventDelete   EventKind = "delete"
)

// LogEvent is emitted by the backend for every file transfer action.
type LogEvent struct {
	Kind      EventKind
	Filename  string
	SizeBytes int64
	At        time.Time
}

// Engine holds the state of our backend server
type Engine struct {
	srv        *http.Server
	cancelFunc context.CancelFunc
	QRData     []byte
	FullURL    string

	// OnEvent is called (from a goroutine) whenever a transfer event occurs.
	// Set this before calling Start. It is safe to update Fyne UI from here
	// using fyne.Do / widget.Refresh patterns inside the callback.
	OnEvent func(LogEvent)
}

// NewEngine creates a new backend instance
func NewEngine() *Engine {
	return &Engine{}
}

// emit fires OnEvent in a separate goroutine so it never blocks HTTP handlers.
func (e *Engine) emit(ev LogEvent) {
	if e.OnEvent != nil {
		go e.OnEvent(ev)
	}
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
		// Stream each file part directly to disk — no RAM buffering at all.
		// The frontend sends one file per request (parallel XHR), so Gin's
		// goroutine-per-request already gives us parallelism for free.
		mr, err := c.Request.MultipartReader()
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid multipart request"})
			return
		}

		// 256 KB I/O buffer: fewer syscalls, saturates disk throughput.
		const bufSize = 256 * 1024
		buf := make([]byte, bufSize)
		count := 0

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			filename := filepath.Base(part.FileName())
			if filename == "" || filename == "." || filename == ".." {
				part.Close()
				continue
			}

			dst, createErr := os.Create(filepath.Join(dir, filename))
			if createErr != nil {
				part.Close()
				continue
			}

			// io.CopyBuffer: network → buf → disk, never fully in RAM.
			written, copyErr := io.CopyBuffer(dst, io.LimitReader(part, 10<<30), buf)
			dst.Close()
			part.Close()
			if copyErr == nil {
				count++
				e.emit(LogEvent{
					Kind:      EventUpload,
					Filename:  filename,
					SizeBytes: written,
					At:        time.Now(),
				})
			}
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
		fullPath := filepath.Join(dir, filepath.Base(name))

		// Stat before serving so we can record the size in the log event.
		info, statErr := os.Stat(fullPath)
		c.FileAttachment(fullPath, filepath.Base(name))
		if statErr == nil {
			e.emit(LogEvent{
				Kind:      EventDownload,
				Filename:  filepath.Base(name),
				SizeBytes: info.Size(),
				At:        time.Now(),
			})
		}
	})

	r.DELETE("/delete", func(c *gin.Context) {
		name := filepath.Base(c.Query("name"))
		fullPath := filepath.Join(dir, name)
		info, statErr := os.Stat(fullPath)
		os.Remove(fullPath)
		c.JSON(200, gin.H{"status": "deleted"})
		if statErr == nil {
			e.emit(LogEvent{
				Kind:      EventDelete,
				Filename:  name,
				SizeBytes: info.Size(),
				At:        time.Now(),
			})
		}
	})

	r.GET("/qr.png", func(c *gin.Context) {
		if len(e.QRData) > 0 {
			c.Data(200, "image/png", e.QRData)
		} else {
			c.Status(404)
		}
	})

	// Serve the embedded web frontend
	subFS, _ := fs.Sub(embeddedFiles, "frontend")
	fileServer := http.FileServer(http.FS(subFS))
	r.NoRoute(func(c *gin.Context) { gin.WrapH(fileServer)(c) })

	e.srv = &http.Server{
		Addr:    "0.0.0.0" + Port,
		Handler: r,
		// No hard timeouts — large file transfers need unlimited time.
		ReadHeaderTimeout: 10 * time.Second, // prevent slow-header attacks
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB header limit
	}

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