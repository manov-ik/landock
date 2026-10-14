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
	"fmt"
	"image/color"
	"net/url"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ncruces/zenity"

	"landock/assets"
	"landock/backend"
)

// ─── Colors ───────────────────────────────────────────────────────────────────

var (
	colorCyan  = color.RGBA{10, 132, 255, 255}
	colorGreen = color.RGBA{52, 199, 89, 255}
	colorRed   = color.RGBA{255, 59, 48, 255}
	colorMuted = color.RGBA{150, 150, 150, 255}
	colorDim   = color.RGBA{140, 140, 140, 255}
	colorWhite = color.White
)

// ─── Chain link SVG icon (curved, Material Design "link") ─────────────────────
// Filled with cyan #0A84FF so it matches the URL color.
var chainLinkSVG = []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><g transform="rotate(-45 12 12)"><path fill="#0A84FF" d="M3.9 12c0-1.71 1.39-3.1 3.1-3.1h4V7H7c-2.76 0-5 2.24-5 5s2.24 5 5 5h4v-1.9H7c-1.71 0-3.1-1.39-3.1-3.1zM8 13h8v-2H8v2zm9-6h-4v1.9h4c1.71 0 3.1 1.39 3.1 3.1s-1.39 3.1-3.1 3.1h-4V17h4c2.76 0 5-2.24 5-5s-2.24-5-5-5z"/></g></svg>`)
var chainLinkRes = fyne.NewStaticResource("chain.svg", chainLinkSVG)

// ─── tappableText ─────────────────────────────────────────────────────────────
// Plain canvas.Text that responds to taps with pointer cursor — no button
// renderer, no hover box, no background.
// IMPORTANT: Tapped is called on the Fyne main goroutine; onTapped must NOT
// be dispatched with `go` or it will violate thread safety.

type tappableText struct {
	widget.BaseWidget
	label     *canvas.Text
	origColor color.Color
	onTapped  func()
}

func newTappableText(text string, col color.Color, size float32, tapped func()) *tappableText {
	l := canvas.NewText(text, col)
	l.TextSize = size
	t := &tappableText{label: l, origColor: col, onTapped: tapped}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableText) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.label)
}

// Direct call — already on main thread, no goroutine needed.
func (t *tappableText) Tapped(_ *fyne.PointEvent) {
	// Brief dim flash on tap — fyne.Do needed in AfterFunc (runs off main thread).
	t.label.Color = colorDim
	t.label.Refresh()
	time.AfterFunc(120*time.Millisecond, func() {
		fyne.Do(func() {
			t.label.Color = t.origColor
			t.label.Refresh()
		})
	})
	if t.onTapped != nil {
		t.onTapped()
	}
}
func (t *tappableText) TappedSecondary(_ *fyne.PointEvent) {}

// Hover: brighten the text to give clear mouse-over feedback.
// Uses .RGBA() which works for any color.Color type (gray, RGBA, etc.).
func (t *tappableText) Cursor() desktop.Cursor { return desktop.PointerCursor }
func (t *tappableText) MouseIn(_ *desktop.MouseEvent) {
	r, g, b, _ := t.origColor.RGBA() // values in [0, 0xffff]
	brighter := color.RGBA{
		R: uint8(min(int(r>>8)+55, 255)),
		G: uint8(min(int(g>>8)+55, 255)),
		B: uint8(min(int(b>>8)+55, 255)),
		A: 255,
	}
	t.label.Color = brighter
	t.label.Refresh()
}
func (t *tappableText) MouseMoved(_ *desktop.MouseEvent) {}
func (t *tappableText) MouseOut() {
	t.label.Color = t.origColor
	t.label.Refresh()
}


// ─── hoverURLLabel ────────────────────────────────────────────────────────────
// URL text in cyan that shows an underline on hover, shows pointer cursor,
// and calls onTapped when clicked (opens in browser).

type hoverURLLabel struct {
	widget.BaseWidget
	label     *canvas.Text
	underline *canvas.Rectangle
	onTapped  func()
}

func newHoverURLLabel(tapped func()) *hoverURLLabel {
	lbl := canvas.NewText("", colorCyan)
	lbl.TextSize = 11

	// Underline always visible; hidden on hover for a clean interactive feel.
	ul := canvas.NewRectangle(colorCyan)
	ul.SetMinSize(fyne.NewSize(0, 1))

	h := &hoverURLLabel{label: lbl, underline: ul, onTapped: tapped}
	h.ExtendBaseWidget(h)
	return h
}

func (h *hoverURLLabel) CreateRenderer() fyne.WidgetRenderer {
	// Stack: label and underline share the same bounding box.
	// The underline border pins to the bottom of that box — no gap below text.
	ulAnchor := container.NewBorder(nil, h.underline, nil, nil)
	return widget.NewSimpleRenderer(container.NewStack(h.label, ulAnchor))
}

func (h *hoverURLLabel) Cursor() desktop.Cursor { return desktop.PointerCursor }
func (h *hoverURLLabel) MouseIn(_ *desktop.MouseEvent) {
	h.underline.FillColor = color.Transparent
	h.underline.Refresh()
}
func (h *hoverURLLabel) MouseMoved(_ *desktop.MouseEvent) {}
func (h *hoverURLLabel) MouseOut() {
	h.underline.FillColor = colorCyan
	h.underline.Refresh()
}

func (h *hoverURLLabel) Tapped(_ *fyne.PointEvent) {
	if h.onTapped != nil {
		h.onTapped()
	}
}
func (h *hoverURLLabel) TappedSecondary(_ *fyne.PointEvent) {}

// ─── tappableIcon ─────────────────────────────────────────────────────────────
// SVG icon that responds to taps, shows pointer cursor, no hover box.

type tappableIcon struct {
	widget.BaseWidget
	res      fyne.Resource
	iconSize fyne.Size
	onTapped func()
	// hover callback receives the mouse event so the caller can position popups.
	onHover func(ev *desktop.MouseEvent, in bool)
}

func newTappableIcon(res fyne.Resource, sz fyne.Size, tapped func(), hover func(*desktop.MouseEvent, bool)) *tappableIcon {
	t := &tappableIcon{res: res, iconSize: sz, onTapped: tapped, onHover: hover}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableIcon) CreateRenderer() fyne.WidgetRenderer {
	img := canvas.NewImageFromResource(t.res)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(t.iconSize)
	return widget.NewSimpleRenderer(img)
}

func (t *tappableIcon) Tapped(_ *fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}
func (t *tappableIcon) TappedSecondary(_ *fyne.PointEvent) {}
func (t *tappableIcon) Cursor() desktop.Cursor { return desktop.PointerCursor }
func (t *tappableIcon) MouseIn(ev *desktop.MouseEvent) {
	if t.onHover != nil { t.onHover(ev, true) }
}
func (t *tappableIcon) MouseMoved(_ *desktop.MouseEvent) {}
func (t *tappableIcon) MouseOut() {
	if t.onHover != nil { t.onHover(nil, false) }
}

// ─── pointerButton ────────────────────────────────────────────────────────────
// Standard Fyne button with pointer cursor. Embeds widget.Button by VALUE
// (not pointer) so ExtendBaseWidget is called exactly once on the outer type.

type pointerButton struct {
	widget.Button
}

func newPointerButton(label string, tapped func()) *pointerButton {
	b := &pointerButton{}
	b.Text = label
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

func (b *pointerButton) Cursor() desktop.Cursor { return desktop.PointerCursor }


type AppState struct {
	isRunning bool
	sharePath string
	engine    *backend.Engine

	mu            sync.Mutex
	logEntries    []backend.LogEvent
	transferCount int
}

func (s *AppState) addEntry(ev backend.LogEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logEntries = append(s.logEntries, ev)
	if ev.Kind == backend.EventUpload || ev.Kind == backend.EventDownload {
		s.transferCount++
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func humanSize(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
}

func eventIcon(kind backend.EventKind) string {
	switch kind {
	case backend.EventUpload:
		return "↑"
	case backend.EventDownload:
		return "↓"
	case backend.EventDelete:
		return "✕"
	default:
		return "•"
	}
}

func eventColor(kind backend.EventKind) color.Color {
	switch kind {
	case backend.EventUpload:
		return colorGreen
	case backend.EventDownload:
		return colorCyan
	case backend.EventDelete:
		return colorRed
	default:
		return colorDim
	}
}

func notifTitle(kind backend.EventKind) string {
	switch kind {
	case backend.EventUpload:
		return "📤 File Received"
	case backend.EventDownload:
		return "📥 File Downloaded"
	case backend.EventDelete:
		return "🗑 File Deleted"
	default:
		return "LAN Dock"
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// vSpacer returns a fixed-height transparent spacer.
func vSpacer(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

func hSpacer(w float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, 0))
	return r
}

// ─── Main ─────────────────────────────────────────────────────────────────────

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

	// ── Log list (shared) ─────────────────────────────────────────────────────
	logList := widget.NewList(
		func() int {
			state.mu.Lock()
			defer state.mu.Unlock()
			return len(state.logEntries)
		},
		func() fyne.CanvasObject {
			icon := canvas.NewText("↑", colorGreen)
			icon.TextSize = 14
			icon.TextStyle = fyne.TextStyle{Bold: true}
			name := canvas.NewText("filename.ext", colorWhite)
			name.TextSize = 13
			meta := canvas.NewText("0 B · 00:00:00", colorDim)
			meta.TextSize = 11
			left := container.NewVBox(name, meta)
			return container.NewBorder(nil, nil,
				container.NewPadded(container.NewCenter(icon)), nil, left)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			state.mu.Lock()
			idx := len(state.logEntries) - 1 - i
			if idx < 0 {
				state.mu.Unlock()
				return
			}
			ev := state.logEntries[idx]
			state.mu.Unlock()

			border := o.(*fyne.Container)
			left := border.Objects[0].(*fyne.Container)
			iconBox := border.Objects[1].(*fyne.Container)

			nameEl := left.Objects[0].(*canvas.Text)
			metaEl := left.Objects[1].(*canvas.Text)
			iconEl := iconBox.Objects[0].(*fyne.Container).Objects[0].(*canvas.Text)

			iconEl.Text = eventIcon(ev.Kind)
			iconEl.Color = eventColor(ev.Kind)
			iconEl.Refresh()
			nameEl.Text = ev.Filename
			nameEl.Refresh()
			metaEl.Text = fmt.Sprintf("%s · %s", humanSize(ev.SizeBytes), ev.At.Format("15:04:05"))
			metaEl.Refresh()
		},
	)

	// ── View switching ────────────────────────────────────────────────────────
	var mainView fyne.CanvasObject
	var logView fyne.CanvasObject
	var root *fyne.Container

	// These are called directly on the Fyne main goroutine (from Tapped).
	showLog := func() {
		mainView.Hide()
		logView.Show()
		logList.Refresh()
	}
	showMain := func() {
		logView.Hide()
		mainView.Show()
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// LOG VIEW
	// ═══════════════════════════════════════════════════════════════════════════

	// "‹ Activity Log" — tappableText in cyan, pointer cursor, no box.
	backBtn := newTappableText("‹  Activity Log", colorWhite, 14, showMain)

	logTopBar := container.NewPadded(
		container.NewBorder(nil, nil, backBtn, nil),
	)

	logView = container.NewBorder(logTopBar, nil, nil, nil,
		container.NewPadded(container.NewPadded(logList)),
	)

	// ═══════════════════════════════════════════════════════════════════════════
	// MAIN VIEW
	// ═══════════════════════════════════════════════════════════════════════════

	// ── Hamburger ☰ top-right, tappableText, no box ───────────────────────────
	hamburger := newTappableText("☰", colorMuted, 20, showLog)
	hamburger.Hide()


	// ── Logo & Title ──────────────────────────────────────────────────────────
	logoRes := fyne.NewStaticResource("logo.png", assets.LogoData)
	logoImg := canvas.NewImageFromResource(logoRes)
	logoImg.FillMode = canvas.ImageFillContain
	logoImg.SetMinSize(fyne.NewSize(64, 64))

	title := canvas.NewText("LAN Dock", colorWhite)
	title.TextSize = 32
	title.TextStyle = fyne.TextStyle{Bold: true}

	// ── Folder path ───────────────────────────────────────────────────────────
	lblHeader := canvas.NewText("Shared Folder", colorMuted)
	lblHeader.TextSize = 12

	lblPath := widget.NewLabel("No folder selected")
	lblPath.Wrapping = fyne.TextTruncate

	// ── QR Code ───────────────────────────────────────────────────────────────
	qrBoxBg := canvas.NewRectangle(colorWhite)
	qrBoxBg.CornerRadius = 18
	qrBoxBg.SetMinSize(fyne.NewSize(240, 240))

	qrBoxText := canvas.NewText("QR", color.Black)
	qrBoxText.TextSize = 18
	qrBoxText.TextStyle = fyne.TextStyle{Bold: true}
	qrPlaceholder := container.NewStack(qrBoxBg, container.NewCenter(qrBoxText))

	qrImg := canvas.NewImageFromResource(nil)
	qrImg.FillMode = canvas.ImageFillContain
	qrImg.SetMinSize(fyne.NewSize(220, 220))
	qrImgContainer := container.NewStack(qrBoxBg, container.NewPadded(qrImg))
	qrImgContainer.Hide()

	qrArea := container.NewStack(qrPlaceholder, qrImgContainer)

	// ── URL row: [chain-link icon] [hoverable URL] ────────────────────────────
	var serverURL *url.URL

	// ── Copy Tooltip ──────────────────────────────────────────────────────────
	tooltipBg := canvas.NewRectangle(color.RGBA{38, 38, 42, 250})
	tooltipBg.CornerRadius = 5
	tooltipBg.StrokeColor = color.RGBA{70, 70, 76, 200}
	tooltipBg.StrokeWidth = 1

	copyTooltipText := canvas.NewText("Copy", colorWhite)
	copyTooltipText.TextSize = 10
	copyTooltipText.Alignment = fyne.TextAlignCenter

	copyTooltip := container.NewStack(
		tooltipBg,
		container.NewCenter(copyTooltipText),
	)
	copyTooltip.Hide()

	const tipW, tipH = float32(54), float32(20)
	var copyHideTimer *time.Timer
	var isCopiedShowing bool

	// Chain link icon — copies URL to clipboard on click.
	var copyLinkIcon *tappableIcon
	showTooltipAtIcon := func(txt string) {
		iconPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(copyLinkIcon)
		iconSz := copyLinkIcon.Size()
		iconW := iconSz.Width
		if iconW == 0 {
			iconW = 16
		}
		iconH := iconSz.Height
		if iconH == 0 {
			iconH = 16
		}

		copyTooltipText.Text = txt
		copyTooltipText.Refresh()
		copyTooltip.Resize(fyne.NewSize(tipW, tipH))
		copyTooltip.Move(fyne.NewPos(iconPos.X+(iconW-tipW)/2, iconPos.Y+iconH+4))
		copyTooltip.Show()
		copyTooltip.Refresh()
	}

	copyLinkIcon = newTappableIcon(chainLinkRes, fyne.NewSize(16, 16),
		func() { // tapped: copy to clipboard + immediately show "Copied" at the exact same position
			if serverURL != nil {
				myWindow.Clipboard().SetContent(serverURL.String())
				if copyHideTimer != nil {
					copyHideTimer.Stop()
				}
				isCopiedShowing = true
				showTooltipAtIcon("Copied")

				copyHideTimer = time.AfterFunc(2*time.Second, func() {
					fyne.Do(func() {
						isCopiedShowing = false
						copyTooltip.Hide()
						copyTooltipText.Text = "Copy"
					})
				})
			}
		},
		func(_ *desktop.MouseEvent, in bool) { // hover: show "Copy" under icon
			if in {
				if !isCopiedShowing {
					showTooltipAtIcon("Copy")
				}
			} else {
				if !isCopiedShowing {
					copyTooltip.Hide()
				}
			}
		})
	copyLinkIcon.Hide()

	// URL label — opens browser when clicked.
	urlHoverLabel := newHoverURLLabel(func() {
		if serverURL != nil {
			_ = fyne.CurrentApp().OpenURL(serverURL)
		}
	})
	urlHoverLabel.Hide()

	urlRow := container.NewHBox(
		layout.NewSpacer(),
		container.NewVBox(vSpacer(4), copyLinkIcon), // push icon down 4px
		urlHoverLabel,
		layout.NewSpacer(),
	)

	// ── Status ────────────────────────────────────────────────────────────────
	statusText := canvas.NewText("Stopped", colorRed)
	statusText.TextSize = 16
	statusText.TextStyle = fyne.TextStyle{Bold: true}
	statusText.Alignment = fyne.TextAlignCenter

	// ── Buttons ───────────────────────────────────────────────────────────────
	var btnStart *pointerButton
	var btnChoose *pointerButton

	updateUI := func(running bool) {
		state.isRunning = running
		if running {
			statusText.Text = "Running"
			statusText.Color = colorGreen
			btnStart.SetText("Stop Server")
			btnStart.Importance = widget.DangerImportance
			btnStart.Refresh()
			btnChoose.Disable()
			qrPlaceholder.Hide()
			qrImgContainer.Show()
			copyLinkIcon.Show()
			urlHoverLabel.Show()
			hamburger.Show()
		} else {
			statusText.Text = "Stopped"
			statusText.Color = colorRed
			btnStart.SetText("Start Server")
			btnStart.Importance = widget.HighImportance
			btnStart.Refresh()
			btnChoose.Enable()
			qrImgContainer.Hide()
			qrPlaceholder.Show()
			copyLinkIcon.Hide()
			copyTooltip.Hide()
			if copyHideTimer != nil {
				copyHideTimer.Stop()
			}
			isCopiedShowing = false
			urlHoverLabel.Hide()
			hamburger.Hide()
			showMain()
		}
		statusText.Refresh()
	}

	// ── Backend events — via fyne.Do (called from HTTP goroutine) ────────────
	state.engine.OnEvent = func(ev backend.LogEvent) {
		state.addEntry(ev)
		fyne.Do(func() {
			myApp.SendNotification(&fyne.Notification{
				Title:   notifTitle(ev.Kind),
				Content: fmt.Sprintf("%s (%s)", ev.Filename, humanSize(ev.SizeBytes)),
			})
			logList.Refresh()
		})
	}

	// ── Folder Selection ──────────────────────────────────────────────────────
	btnChoose = newPointerButton("Choose Folder", func() {
		dir, err := zenity.SelectFile(
			zenity.Title("Select Folder to Share"),
			zenity.Directory(),
		)
		if err == nil && dir != "" {
			state.sharePath = dir
			lblPath.SetText(dir)
		}
	})

	// ── Server Start/Stop ─────────────────────────────────────────────────────
	btnStart = newPointerButton("Start Server", func() {
		if state.isRunning {
			state.engine.Stop()
			updateUI(false)
		} else {
			if state.sharePath == "" {
				zenity.Error("Please select a folder first.")
				return
			}
			fullURL, qrDataBytes, err := state.engine.Start(state.sharePath)
			if err != nil {
				zenity.Error("Server Error: " + err.Error())
				return
			}

			res := fyne.NewStaticResource("qr.png", qrDataBytes)
			qrImg.Resource = res
			qrImg.Refresh()

			serverURL, _ = url.Parse(fullURL)
			urlHoverLabel.label.Text = fullURL
			urlHoverLabel.label.Refresh()

			// Reset session log
			state.mu.Lock()
			state.logEntries = nil
			state.transferCount = 0
			state.mu.Unlock()
			logList.Refresh()

			updateUI(true)
		}
	})
	btnStart.Importance = widget.HighImportance

	// ── Main content ──────────────────────────────────────────────────────────
	// vSpacer(6) at the top closes the gap between ☰ and the logo.
	mainContent := container.NewVBox(
		vSpacer(6),
		container.NewCenter(container.NewHBox(logoImg, title)),
		layout.NewSpacer(),
		container.NewVBox(lblHeader, container.NewPadded(lblPath), btnChoose),
		layout.NewSpacer(),
		container.NewCenter(qrArea),
		layout.NewSpacer(),
		container.NewCenter(urlRow),
		layout.NewSpacer(),
		container.NewCenter(statusText),
		layout.NewSpacer(),
		btnStart,
	)

	// ── Main view: hamburger floats in a Stack overlay so it takes
	// zero space in the layout — logo position never moves.
	// VBox(hamburger, spacer) pins the hamburger to the TOP of the right column.
	mainView = container.NewStack(
		container.NewPadded(container.NewPadded(mainContent)),
		container.NewBorder(nil, nil, nil,
			container.NewVBox(
				container.NewHBox(hamburger, hSpacer(10)), // extra right padding
				layout.NewSpacer())),
		container.NewWithoutLayout(copyTooltip),
	)

	// ── Root stack ────────────────────────────────────────────────────────────
	logView.Hide()
	root = container.NewStack(mainView, logView)

	myWindow.SetCloseIntercept(func() {
		if state.isRunning {
			state.engine.Stop()
		}
		myWindow.Close()
	})

	myWindow.SetContent(root)
	myWindow.ShowAndRun()
}

// ─── Custom Theme ─────────────────────────────────────────────────────────────

type customTheme struct{ fyne.Theme }

func (m *customTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Bold {
		return fyne.NewStaticResource("SpaceMono-Bold.ttf", assets.FontBold)
	}
	return fyne.NewStaticResource("SpaceMono-Regular.ttf", assets.FontRegular)
}