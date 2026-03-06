package stats

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/config"
)

// TopHandler handles the !top command.
type TopHandler struct {
	service *Service
	config  config.StatsConfig
	logger  *slog.Logger
}

// NewTopHandler creates a new top command handler.
func NewTopHandler(service *Service, cfg config.StatsConfig, logger *slog.Logger) *TopHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TopHandler{service: service, config: cfg, logger: logger}
}

// Command returns the command name for Telegram registration.
func (h *TopHandler) Command() string {
	return "top"
}

// Description returns the command description for Telegram registration.
func (h *TopHandler) Description() string {
	return "Show top users by message count"
}

// Handle processes the !top command.
func (h *TopHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	chatID := msg.Chat.ID
	if msg.Text == "" {
		return
	}

	limit, err := extractTopLimit(msg.Text, h.config.TopDefaultLimit, h.config.TopMaxLimit)
	if err != nil {
		h.reply(ctx, b, chatID, err.Error())
		return
	}

	h.logger.Info("executing top command", "chat_id", chatID, "limit", limit)

	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekStart := mondayOf(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	today, errToday := h.service.GetTopUsersSince(ctx, chatID, todayStart, limit)
	week, errWeek := h.service.GetTopUsersSince(ctx, chatID, weekStart, limit)
	month, errMonth := h.service.GetTopUsersSince(ctx, chatID, monthStart, limit)
	total, errTotal := h.service.GetTopUsersTotal(ctx, chatID, limit)

	for _, e := range []error{errToday, errWeek, errMonth, errTotal} {
		if e != nil {
			h.logger.Error("failed to load top users", "error", e)
			h.reply(ctx, b, chatID, "Failed to load top users.")
			return
		}
	}

	text := formatTopResults(limit, todayStart, weekStart, monthStart, today, week, month, total)
	h.reply(ctx, b, chatID, text)
}

func (h *TopHandler) reply(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		h.logger.Error("failed to send message", "error", err)
	}
}

// extractTopLimit parses the optional numeric argument from the command text.
// Returns the configured default if no argument is given, or an error for
// invalid input.
func extractTopLimit(text string, defaultLimit, maxLimit int) (int, error) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return defaultLimit, nil
	}

	n, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("Usage: !top [number]")
	}
	if n < 1 {
		return 0, fmt.Errorf("Usage: !top [number] (minimum 1)")
	}
	if n > maxLimit {
		return 0, fmt.Errorf("Maximum is %d.", maxLimit)
	}
	return n, nil
}

// mondayOf returns the Monday 00:00 UTC of the week containing t.
func mondayOf(t time.Time) time.Time {
	t = t.UTC()
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := int(weekday) - int(time.Monday)
	monday := t.AddDate(0, 0, -daysSinceMonday)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

func formatTopResults(
	limit int,
	todayStart, weekStart, monthStart time.Time,
	today, week, month, total []TopUser,
) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Top %d users\n", limit)

	writeSection(&sb, fmt.Sprintf("\nToday (%s):", todayStart.Format("02 Jan")), today)
	writeSection(&sb, fmt.Sprintf("\nThis week (from %s):", weekStart.Format("02 Jan")), week)
	writeSection(&sb, fmt.Sprintf("\nThis month (%s):", monthStart.Format("Jan 2006")), month)
	writeSection(&sb, "\nAll time:", total)

	return sb.String()
}

func writeSection(sb *strings.Builder, header string, users []TopUser) {
	fmt.Fprintln(sb, header)
	if len(users) == 0 {
		fmt.Fprintln(sb, "  No messages yet.")
		return
	}
	for i, u := range users {
		name := u.UserName
		if name == "" {
			name = "(unknown)"
		}
		fmt.Fprintf(sb, "  %d. %s — %d\n", i+1, name, u.MessageCount)
	}
}
