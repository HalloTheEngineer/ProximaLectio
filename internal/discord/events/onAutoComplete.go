package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func (h *Handler) AutocompleteListener(e *events.AutocompleteInteractionCreate) {

	switch e.Data.CommandName {
	case "login":
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			if err := e.AutocompleteResult(h.buildSchoolAutocomplete(ctx, e)); err != nil {
				slog.Error(err.Error())
			}
		}()
	case "theme":
		if err := e.AutocompleteResult(h.buildThemeAutocomplete(e)); err != nil {
			slog.Error(err.Error())
		}
	case "room":
		go func() {
			if err := e.AutocompleteResult(h.buildRoomAutocomplete(e)); err != nil {
				slog.Error(err.Error())
			}
		}()
	}

}

func (h *Handler) buildSchoolAutocomplete(ctx context.Context, e *events.AutocompleteInteractionCreate) (choices []discord.AutocompleteChoice) {
	var query string
	focused := e.AutocompleteInteraction.Data.Focused()

	if focused.Name != "institution" {
		return []discord.AutocompleteChoice{
			discord.AutocompleteChoiceString{
				Name:  "Error",
				Value: "error",
			},
		}

	}

	data, err := json.Marshal(focused.Value)
	if err != nil {
		return []discord.AutocompleteChoice{}
	}
	query = strings.Trim(string(data), "\"")

	if !(len(query) > 5) {
		return []discord.AutocompleteChoice{}
	}

	schools, err := h.DB.Untis.SearchSchools(ctx, query, "")
	if err != nil {
		return []discord.AutocompleteChoice{}
	}

	for _, school := range schools {
		choices = append(choices, discord.AutocompleteChoiceString{
			Name:  school.DisplayName,
			Value: school.TenantId,
		})
	}

	if len(choices) > 15 {
		choices = choices[:15]
	}
	return
}

func (h *Handler) buildThemeAutocomplete(e *events.AutocompleteInteractionCreate) (choices []discord.AutocompleteChoice) {
	var query string
	focused := e.AutocompleteInteraction.Data.Focused()

	choices = append(choices, discord.AutocompleteChoiceString{
		Name:  "DEFAULT",
		Value: "DEFAULT",
	})

	data, err := json.Marshal(focused.Value)
	if err != nil {
		return []discord.AutocompleteChoice{}
	}
	query = strings.Trim(string(data), "\"")

	themes, err := h.DB.Untis.GetThemes(query)
	if err != nil {
		return choices
	}

	for _, theme := range themes {
		if theme == nil {
			continue
		}
		choices = append(choices, discord.AutocompleteChoiceString{
			Name:  *theme,
			Value: *theme,
		})
	}

	if len(choices) > 20 {
		choices = choices[:20]
	}
	return
}
func (h *Handler) buildRoomAutocomplete(e *events.AutocompleteInteractionCreate) []discord.AutocompleteChoice {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var query string
	focused := e.AutocompleteInteraction.Data.Focused()

	data, err := json.Marshal(focused.Value)
	if err != nil {
		return []discord.AutocompleteChoice{}
	}
	query = strings.Trim(string(data), "\"")

	subjects, err := h.DB.Untis.GetUniqueSubjects(ctx, e.User().ID.String(), query)
	if err != nil || subjects == nil || len(subjects) == 0 {
		return nil
	}

	var choices = make([]discord.AutocompleteChoice, len(subjects))

	for i, sub := range subjects {
		choices[i] = discord.AutocompleteChoiceString{Name: sub, Value: sub}
	}

	return choices
}
