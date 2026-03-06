package stats

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	internalBot "github.com/graffic/wanon-go/internal/bot"
)

// SeenHandler handles the /seen and !seen commands.
type SeenHandler struct {
	internalBot.BaseHandler
	service *Service
}

// NewSeenHandler creates a new seen command handler.
func NewSeenHandler(b internalBot.BotClient, service *Service, logger *slog.Logger) *SeenHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SeenHandler{BaseHandler: internalBot.NewBaseHandler(b, logger), service: service}
}

func (h *SeenHandler) Command() string {
	return "seen"
}

func (h *SeenHandler) Description() string {
	return "Show last seen and message stats for a user"
}

// Handle processes the /seen and !seen commands.
func (h *SeenHandler) Handle(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	chatID := msg.Chat.ID
	if msg.Text == "" {
		return
	}

	userName, ok := extractSeenTarget(msg.Text)
	if !ok {
		h.Reply(ctx, chatID, "Usage: /seen @username")
		return
	}

	h.Logger.Info("executing seen command", "chat_id", chatID, "user_name", userName)

	stats, err := h.service.GetUserStatsByName(ctx, chatID, userName)
	if err != nil {
		h.Logger.Error("failed to load user stats", "error", err)
		h.Reply(ctx, chatID, "Failed to load stats for that user.")
		return
	}
	if stats == nil {
		h.Reply(ctx, chatID, fmt.Sprintf("No stats found for @%s.", userName))
		return
	}

	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := dayStart.AddDate(0, 0, -9)
	end := dayStart.AddDate(0, 0, 1)

	counts, err := h.service.GetUserDailyCounts(ctx, chatID, stats.UserID, start, end)
	if err != nil {
		h.Logger.Error("failed to load daily counts", "error", err)
		h.Reply(ctx, chatID, "Failed to load stats for that user.")
		return
	}

	chart := renderDailyChart(start, 10, counts)
	lastSeen := stats.LastMessageAt.UTC().Format("2006-01-02 15:04 MST")
	name := stats.UserName
	if name == "" {
		name = userName
	}

	text := fmt.Sprintf("User: %s\nLast seen: %s\nTotal messages: %d\n\nMessages (last 10 days):\n%s",
		name,
		lastSeen,
		stats.TotalMessages,
		chart,
	)

	h.Reply(ctx, chatID, text)
}

func extractSeenTarget(text string) (string, bool) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return "", false
	}
	userName := strings.TrimPrefix(fields[1], "@")
	userName = strings.TrimSpace(userName)
	if userName == "" {
		return "", false
	}
	return userName, true
}

func renderDailyChart(start time.Time, days int, counts []DailyCount) string {
	countMap := make(map[string]int64, days)
	var maxCount int64

	for _, entry := range counts {
		day := entry.Day.UTC()
		dayKey := day.Format("2006-01-02")
		countMap[dayKey] = entry.MessageCount
		if entry.MessageCount > maxCount {
			maxCount = entry.MessageCount
		}
	}

	lines := make([]string, 0, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		dayKey := day.Format("2006-01-02")
		count := countMap[dayKey]
		bars := renderBars(count, maxCount, 10)
		lines = append(lines, fmt.Sprintf("%s | %-10s %d", dayKey, bars, count))
	}

	return strings.Join(lines, "\n")
}

func renderBars(count, maxCount int64, maxBars int) string {
	if count == 0 || maxCount == 0 {
		return ""
	}

	barLen := int(math.Round(float64(count) / float64(maxCount) * float64(maxBars)))
	if barLen == 0 {
		barLen = 1
	}

	return strings.Repeat("#", barLen)
}
