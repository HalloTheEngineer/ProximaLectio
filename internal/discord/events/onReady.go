package events

import (
	"log/slog"

	"github.com/disgoorg/disgo/events"
)

func (h *Handler) OnReady(e *events.Ready) {
	slog.Info("(✓) Bot logged in;", "user", e.User.Tag())
}
