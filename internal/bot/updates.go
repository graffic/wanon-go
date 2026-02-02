package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/telegram"
)

// Updates handles polling for Telegram updates
type Updates struct {
	client  telegram.Client
	outCh   chan<- []models.Update
}

// NewUpdates creates a new update poller
func NewUpdates(client telegram.Client, outCh chan<- []models.Update) *Updates {
	return &Updates{
		client: client,
		outCh:  outCh,
	}
}

// Start polls for updates in a loop until context is cancelled
func (u *Updates) Start(ctx context.Context) error {
	slog.Info("starting update poller")

	offset := 0
	limit := 100
	timeout := 10

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping update poller")
			return ctx.Err()
		default:
		}

		updates, err := u.client.GetUpdates(ctx, offset, limit, timeout)
		if err != nil {
			slog.Error("failed to get updates", "error", err)
			continue
		}

		if len(updates) > 0 {
			slog.Debug("received updates", "count", len(updates))

			// Send updates to the dispatcher via channel
			select {
			case <-ctx.Done():
				return ctx.Err()
			case u.outCh <- updates:
			}

			// Update offset to acknowledge received updates
			offset = lastUpdateID(updates) + 1
		}
	}
}

// lastUpdateID returns the highest update ID from a batch
func lastUpdateID(updates []models.Update) int {
	if len(updates) == 0 {
		return 0
	}

	maxID := updates[0].ID
	for _, u := range updates[1:] {
		if u.ID > maxID {
			maxID = u.ID
		}
	}
	return int(maxID)
}
