package services

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	api "proximaLectio/internal/untis"
)

type ThemeService struct {
	db *sql.DB
}

func NewThemeService(db *sql.DB) *ThemeService {
	return &ThemeService{db: db}
}

func (s *ThemeService) GetThemes(filter string) ([]*string, error) {
	entries, _ := os.ReadDir(ThemeFolder)
	var l []*string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && strings.Contains(strings.ToLower(e.Name()), strings.ToLower(filter)) {
			str := strings.ToUpper(strings.TrimSuffix(e.Name(), ".json"))
			l = append(l, &str)
		}
	}
	return l, nil
}

func (s *ThemeService) GetTheme(themeID string) (*Theme, error) {
	id := strings.ToLower(strings.TrimSpace(themeID))

	if id == "" || id == "default" {
		def := DefaultRenderConfig().Theme
		return &def, nil
	}

	theme, err := LoadTheme(id)
	if err != nil {
		return nil, fmt.Errorf("theme '%s' not found: %w", themeID, err)
	}

	return &theme, nil
}

func (s *ThemeService) SetTheme(ctx context.Context, id string, themeID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET theme_id = $1 WHERE id = $2`, themeID, id)
	return err
}

type RenderService struct {
	db       *sql.DB
	themeSvc *ThemeService
}

func NewRenderService(db *sql.DB, themeSvc *ThemeService) *RenderService {
	return &RenderService{db: db, themeSvc: themeSvc}
}

func (s *RenderService) GenerateScheduleImage(timetable *api.TimetableEntry, daysCount int, themeID string) (io.Reader, error) {
	config := DefaultRenderConfig()
	config.DaysCount = daysCount
	if themeID != "" && themeID != "default" {
		if t, err := s.themeSvc.GetTheme(themeID); err == nil {
			config.Theme = *t
		}
	}
	if len(timetable.Days) > 0 {
		if date, err := time.Parse("2006-01-02", timetable.Days[0].Date); err == nil {
			config.StartDayName = getGermanDayName(date.Weekday())
		}
	}
	items := s.mapTimetableToItems(timetable, config.Theme)
	renderer := NewCanvasRenderer(config, items)
	return renderer.Draw()
}

func getGermanDayName(weekday time.Weekday) string {
	dayNames := map[time.Weekday]string{
		time.Monday:    "MONTAG",
		time.Tuesday:   "DIENSTAG",
		time.Wednesday: "MITTWOCH",
		time.Thursday:  "DONNERSTAG",
		time.Friday:    "FREITAG",
		time.Saturday:  "SAMSTAG",
		time.Sunday:    "SONNTAG",
	}
	return dayNames[weekday]
}

func (s *RenderService) mapTimetableToItems(timetable *api.TimetableEntry, theme Theme) []RenderItem {
	var items []RenderItem
	for i, day := range timetable.Days {
		for _, slot := range day.GridEntries {
			sH, sM := parseTime(slot.Duration.Start)
			eH, eM := parseTime(slot.Duration.End)

			status := slot.Status
			if status == "CHANGED" {
				status = "SUBSTITUTION"
			}

			item := RenderItem{
				DayIndex:         i,
				StartH:           sH,
				StartM:           sM,
				EndH:             eH,
				EndM:             eM,
				Color:            theme.RegularBg,
				TextColor:        theme.RegularText,
				Status:           status,
				SubstitutionText: slot.SubstitutionText,
			}

			switch status {
			case "SUBSTITUTION":
				item.Color, item.TextColor = theme.SubstitutionBg, theme.SubstitutionText
			case "CANCELLED":
				item.Color, item.TextColor = theme.CancelledBg, theme.CancelledText
			case "EXAM":
				item.Color, item.TextColor = theme.ExamBg, theme.ExamText
			}

			allPositions := [][]api.Position{slot.Position1, slot.Position2, slot.Position3, slot.Position4, slot.Position5, slot.Position6, slot.Position7}
			for _, list := range allPositions {
				for _, pos := range list {
					if pos.Current != nil {
						switch pos.Current.Type {
						case "SUBJECT":
							item.Title = pos.Current.ShortName
						case "ROOM":
							item.Room = pos.Current.ShortName
						case "TEACHER":
							item.Teacher = pos.Current.ShortName
						}
					}
				}
			}
			if item.Title == "" {
				item.Title = "—"
			}
			if item.Room == "" {
				item.Room = "—"
			}
			if item.Teacher == "" {
				item.Teacher = "—"
			}
			items = append(items, item)
		}
	}
	return items
}
