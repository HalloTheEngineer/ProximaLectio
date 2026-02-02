package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
)

var (
	cachedFontFamily *canvas.FontFamily
	fontLoaderOnce   sync.Once

	backgroundImageCache = make(map[string]image.Image)
	imageCacheMu         sync.RWMutex

	fontFaceCache = make(map[string]*canvas.FontFace)
	fontFaceMu    sync.Mutex
)

type RenderItem struct {
	DayIndex  int
	StartH    int
	StartM    int
	EndH      int
	EndM      int
	Color     color.RGBA
	TextColor color.RGBA
	Title     string
	Subtitle  string
	Footer    string
	Status    string
}

type Theme struct {
	Name             string     `json:"name"`
	BackgroundPath   string     `json:"background_path"`
	BackgroundColor  color.RGBA `json:"background_color"`
	HeaderColor      color.RGBA `json:"header_color"`
	SidebarColor     color.RGBA `json:"sidebar_color"`
	GridColor        color.RGBA `json:"grid_color"`
	DayNameColor     color.RGBA `json:"day_name_color"`
	TimeLabelColor   color.RGBA `json:"time_label_color"`
	RegularBg        color.RGBA `json:"regular_bg"`
	RegularText      color.RGBA `json:"regular_text"`
	SubstitutionBg   color.RGBA `json:"substitution_bg"`
	SubstitutionText color.RGBA `json:"substitution_text"`
	CancelledBg      color.RGBA `json:"cancelled_bg"`
	CancelledText    color.RGBA `json:"cancelled_text"`
	RoomChangeBg     color.RGBA `json:"room_change_bg"`
	RoomChangeText   color.RGBA `json:"room_change_text"`
	ExamBg           color.RGBA `json:"exam_bg"`
	ExamText         color.RGBA `json:"exam_text"`
	OfficeHourBg     color.RGBA `json:"office_hour_bg"`
	OfficeHourText   color.RGBA `json:"office_hour_text"`
	AdditionalBg     color.RGBA `json:"additional_bg"`
	AdditionalText   color.RGBA `json:"additional_text"`
	StatusTextColor  color.RGBA `json:"status_text_color"`
}

type RenderConfig struct {
	DayHeight     int
	HourWidth     int
	TimeRowHeight int
	DayColWidth   int
	Margin        int
	DaysCount     int
	HoursCount    int
	PivotHour     int
	PivotMinute   int
	FontSizeScale float64
	Theme         Theme
	DPMM          float64
}

func DefaultRenderConfig() RenderConfig {
	return RenderConfig{
		DayHeight:     350,
		HourWidth:     450,
		TimeRowHeight: 150,
		DayColWidth:   250,
		Margin:        25,
		DaysCount:     5,
		HoursCount:    11,
		PivotHour:     7,
		PivotMinute:   0,
		FontSizeScale: 0.7,
		DPMM:          0.8,
		Theme: Theme{
			Name:             "Default",
			BackgroundColor:  color.RGBA{R: 15, G: 23, B: 42, A: 255},
			HeaderColor:      color.RGBA{R: 30, G: 41, B: 59, A: 220},
			SidebarColor:     color.RGBA{R: 30, G: 41, B: 59, A: 180},
			GridColor:        color.RGBA{R: 255, G: 255, B: 255, A: 20},
			DayNameColor:     color.RGBA{R: 248, G: 250, B: 252, A: 255},
			TimeLabelColor:   color.RGBA{R: 255, G: 255, B: 255, A: 100},
			RegularBg:        color.RGBA{R: 51, G: 65, B: 85, A: 245},
			RegularText:      color.RGBA{R: 241, G: 245, B: 249, A: 255},
			SubstitutionBg:   color.RGBA{R: 180, G: 83, B: 9, A: 245},
			SubstitutionText: color.RGBA{R: 255, G: 251, B: 235, A: 255},
			CancelledBg:      color.RGBA{R: 159, G: 18, B: 57, A: 245},
			CancelledText:    color.RGBA{R: 255, G: 241, B: 242, A: 255},
			RoomChangeBg:     color.RGBA{R: 7, G: 89, B: 133, A: 245},
			RoomChangeText:   color.RGBA{R: 240, G: 249, B: 255, A: 255},
			ExamBg:           color.RGBA{R: 107, G: 33, B: 168, A: 245},
			ExamText:         color.RGBA{R: 250, G: 245, B: 255, A: 255},
			OfficeHourBg:     color.RGBA{R: 21, G: 128, B: 61, A: 245},
			OfficeHourText:   color.RGBA{R: 240, G: 253, B: 244, A: 255},
			AdditionalBg:     color.RGBA{R: 71, G: 85, B: 105, A: 245},
			AdditionalText:   color.RGBA{R: 241, G: 245, B: 249, A: 255},
			StatusTextColor:  color.RGBA{R: 255, G: 255, B: 255, A: 150},
		},
	}
}

func LoadTheme(themeID string) (Theme, error) {
	var theme Theme
	path := fmt.Sprintf("assets/themes/%s.json", themeID)
	data, err := os.ReadFile(path)
	if err != nil {
		return theme, err
	}
	err = json.Unmarshal(data, &theme)
	return theme, err
}

type CanvasRenderer struct {
	items  []RenderItem
	config RenderConfig
	family *canvas.FontFamily
}

func PreWarmRenderer() {
	_ = NewCanvasRenderer(DefaultRenderConfig(), nil)
}

func NewCanvasRenderer(config RenderConfig, items []RenderItem) *CanvasRenderer {
	fontLoaderOnce.Do(func() {
		cachedFontFamily = canvas.NewFontFamily("Inter")
		load := func(p string, s canvas.FontStyle) {
			if d, err := os.ReadFile(p); err == nil {
				_ = cachedFontFamily.LoadFont(d, 0, s)
			}
		}
		load("assets/fonts/Inter-Regular.ttf", canvas.FontRegular)
		load("assets/fonts/Inter-Bold.ttf", canvas.FontBold)
	})
	return &CanvasRenderer{items: items, config: config, family: cachedFontFamily}
}

func (r *CanvasRenderer) getFace(size float64, col color.Color, style canvas.FontStyle) *canvas.FontFace {
	fontFaceMu.Lock()
	defer fontFaceMu.Unlock()
	r1, g1, b1, a1 := col.RGBA()
	key := fmt.Sprintf("%.2f-%d-%d-%d-%d-%d", size, style, r1, g1, b1, a1)
	if face, ok := fontFaceCache[key]; ok {
		return face
	}
	face := r.family.Face(size, col, style, canvas.FontNormal)
	fontFaceCache[key] = face
	return face
}

func (r *CanvasRenderer) getBackgroundImage(path string) image.Image {
	imageCacheMu.RLock()
	img, ok := backgroundImageCache[path]
	imageCacheMu.RUnlock()
	if ok {
		return img
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	decoded, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	imageCacheMu.Lock()
	backgroundImageCache[path] = decoded
	imageCacheMu.Unlock()
	return decoded
}

func (r *CanvasRenderer) Draw() (io.Reader, error) {
	width := math.Round(float64(r.config.DayColWidth + (r.config.HourWidth * r.config.HoursCount)))
	height := math.Round(float64(r.config.TimeRowHeight + (r.config.DayHeight * r.config.DaysCount)))

	c := canvas.New(width, height)
	ctx := canvas.NewContext(c)
	r.renderToContext(ctx, width, height)

	var buf bytes.Buffer
	if err := c.Write(&buf, renderers.PNG(
		canvas.DPMM(r.config.DPMM),
		png.Encoder{CompressionLevel: png.BestSpeed},
	)); err != nil {
		return nil, err
	}
	return &buf, nil
}

func (r *CanvasRenderer) renderToContext(ctx *canvas.Context, width, height float64) {
	ctx.SetFillColor(r.config.Theme.BackgroundColor)
	ctx.DrawPath(0, 0, canvas.Rectangle(width, height))

	if r.config.Theme.BackgroundPath != "" {
		if img := r.getBackgroundImage(r.config.Theme.BackgroundPath); img != nil {
			imgW, imgH := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
			dpmm := math.Min(imgW/width, imgH/height)
			offsetX := (width - (imgW / dpmm)) / 2
			offsetY := (height - (imgH / dpmm)) / 2
			ctx.DrawImage(offsetX, offsetY, img, canvas.DPMM(dpmm))
		}
	}

	ctx.SetFillColor(r.config.Theme.SidebarColor)
	ctx.DrawPath(0, 0, canvas.Rectangle(float64(r.config.DayColWidth), height-float64(r.config.TimeRowHeight)))

	ctx.SetFillColor(r.config.Theme.HeaderColor)
	ctx.DrawPath(0, height-float64(r.config.TimeRowHeight), canvas.Rectangle(width, float64(r.config.TimeRowHeight)))

	labelScale := r.config.FontSizeScale * 2.0
	fLabel := r.getFace(64.0*labelScale, r.config.Theme.TimeLabelColor, canvas.FontBold)
	fDay := r.getFace(52.0*labelScale, r.config.Theme.DayNameColor, canvas.FontBold)

	ctx.SetStrokeColor(r.config.Theme.GridColor)
	ctx.SetStrokeWidth(2.0)
	for i := 0; i <= r.config.HoursCount; i++ {
		x := math.Round(float64(r.config.DayColWidth + (i * r.config.HourWidth)))
		ctx.MoveTo(x, height)
		ctx.LineTo(x, height-float64(r.config.TimeRowHeight))
		ctx.Stroke()
		label := fmt.Sprintf("%02d:00", r.config.PivotHour+i)
		ctx.DrawText(x, height-(float64(r.config.TimeRowHeight)/2)-10, canvas.NewTextLine(fLabel, label, canvas.Center))
	}

	for i := 0; i < r.config.DaysCount; i++ {
		yT := height - float64(r.config.TimeRowHeight) - float64(i*r.config.DayHeight)
		yC := yT - (float64(r.config.DayHeight) / 2)
		dayNames := []string{"MONTAG", "DIENSTAG", "MITTWOCH", "DONNERSTAG", "FREITAG", "SAMSTAG", "SONNTAG"}
		ctx.DrawText(float64(r.config.DayColWidth)/2, yC-10, canvas.NewTextLine(fDay, dayNames[i%7], canvas.Center))
		ctx.SetStrokeColor(r.config.Theme.GridColor)
		ctx.SetStrokeWidth(2.0)
		ctx.MoveTo(0, yT)
		ctx.LineTo(width, yT)
		ctx.Stroke()
	}

	textScale := r.config.FontSizeScale * 2.2
	for _, item := range r.items {
		if item.DayIndex >= r.config.DaysCount {
			continue
		}
		yRowTop := height - float64(r.config.TimeRowHeight) - float64(item.DayIndex*r.config.DayHeight)
		relMin := (item.StartH-r.config.PivotHour)*60 + (item.StartM - r.config.PivotMinute)
		durMin := (item.EndH-item.StartH)*60 + (item.EndM - item.StartM)

		xS := math.Round(float64(r.config.DayColWidth) + (float64(relMin) * (float64(r.config.HourWidth) / 60.0)))
		iW := math.Round((float64(durMin) * (float64(r.config.HourWidth) / 60.0)) - float64(r.config.Margin))
		bH := math.Round(float64(r.config.DayHeight) - float64(r.config.Margin*2))
		yB := math.Round(yRowTop - bH - float64(r.config.Margin))

		radius := 24.0
		cardPath := canvas.RoundedRectangle(iW, bH, radius)

		ctx.SetFillColor(item.Color)
		ctx.SetStrokeColor(item.Color)
		ctx.SetStrokeWidth(1.0)
		ctx.DrawPath(xS, yB, cardPath)

		ctx.SetFillColor(color.RGBA{0, 0, 0, 0})
		ctx.SetStrokeColor(color.RGBA{255, 255, 255, 45})
		ctx.SetStrokeWidth(1.5)
		ctx.DrawPath(xS, yB, cardPath)

		fT := r.getFace(90.0*textScale, item.TextColor, canvas.FontBold)
		fD := r.getFace(55.0*textScale, color.RGBA{R: item.TextColor.R, G: item.TextColor.G, B: item.TextColor.B, A: 215}, canvas.FontBold)
		fS := r.getFace(45.0*textScale, color.RGBA{R: item.TextColor.R, G: item.TextColor.G, B: item.TextColor.B, A: 185}, canvas.FontRegular)

		textPadding := 45.0
		ctx.DrawText(xS+textPadding, yRowTop-110, canvas.NewTextLine(fT, item.Title, canvas.Left))
		ctx.DrawText(xS+textPadding, yRowTop-195, canvas.NewTextLine(fD, item.Footer, canvas.Left))
		ctx.DrawText(xS+textPadding, yRowTop-275, canvas.NewTextLine(fS, item.Subtitle, canvas.Left))

		fTime := r.getFace(50.0*textScale, color.RGBA{R: item.TextColor.R, G: item.TextColor.G, B: item.TextColor.B, A: 230}, canvas.FontBold)
		ctx.DrawText(xS+iW-40, yRowTop-90, canvas.NewTextLine(fTime, fmt.Sprintf("%02d:%02d", item.StartH, item.StartM), canvas.Right))

		if item.Status != "REGULAR" && item.Status != "" {
			fStat := r.getFace(40.0*textScale, r.config.Theme.StatusTextColor, canvas.FontBold)
			ctx.DrawText(xS+iW-40, yB+45, canvas.NewTextLine(fStat, strings.ToUpper(item.Status), canvas.Right))
		}
	}
}
