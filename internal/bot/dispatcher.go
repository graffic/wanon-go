package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/telegram"
)

// UpdateHandler is a function that processes updates before command execution
// Used for middleware like caching
type UpdateHandler func(ctx context.Context, update *models.Update) error

// Dispatcher handles incoming updates and routes them to appropriate commands
type Dispatcher struct {
	inCh           <-chan []models.Update
	commands       map[string]Command
	allowedChatIDs map[int64]bool
	updateHandlers []UpdateHandler
}

// NewDispatcher creates a new update dispatcher
func NewDispatcher(inCh <-chan []models.Update, allowedChatIDs []int64) *Dispatcher {
	allowed := make(map[int64]bool, len(allowedChatIDs))
	for _, id := range allowedChatIDs {
		allowed[id] = true
	}

	return &Dispatcher{
		inCh:           inCh,
		commands:       make(map[string]Command),
		allowedChatIDs: allowed,
		updateHandlers: make([]UpdateHandler, 0),
	}
}

// AddUpdateHandler registers a handler that processes all updates before command execution
// This is used for middleware like caching messages
func (d *Dispatcher) AddUpdateHandler(handler UpdateHandler) {
	d.updateHandlers = append(d.updateHandlers, handler)
	slog.Info("registered update handler")
}

// Register adds a command to the dispatcher
func (d *Dispatcher) Register(name string, cmd Command) {
	d.commands[name] = cmd
	slog.Info("registered command", "name", name)
}

// Start processes updates from the channel until context is cancelled
func (d *Dispatcher) Start(ctx context.Context) error {
	slog.Info("starting dispatcher")

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping dispatcher")
			return ctx.Err()
		case updates := <-d.inCh:
			d.processUpdates(ctx, updates)
		}
	}
}

// processUpdates handles a batch of updates
func (d *Dispatcher) processUpdates(ctx context.Context, updates []models.Update) {
	for _, update := range updates {
		// Run update handlers (e.g., cache middleware) first
		for _, handler := range d.updateHandlers {
			if err := handler(ctx, &update); err != nil {
				slog.Error("update handler failed", "error", err)
				// Continue processing even if handler fails
			}
		}

		// Extract message from update (handle both regular and edited messages)
		var msg *models.Message
		if update.Message != nil {
			msg = update.Message
		} else if update.EditedMessage != nil {
			msg = update.EditedMessage
		}

		if msg == nil {
			continue
		}

		// Check if chat is allowed
		if !d.isChatAllowed(msg.Chat.ID) {
			slog.Debug("ignoring message from unauthorized chat", "chat_id", msg.Chat.ID)
			continue
		}

		// Extract command from message text
		cmdName := extractCommand(msg.Text)
		if cmdName == "" {
			continue
		}

		// Find and execute command
		cmd, ok := d.commands[cmdName]
		if !ok {
			slog.Debug("unknown command", "command", cmdName)
			continue
		}

		slog.Info("executing command", "command", cmdName, "chat_id", msg.Chat.ID)
		if err := cmd.Execute(ctx, msg); err != nil {
			slog.Error("command execution failed", "command", cmdName, "error", err)
		}
	}
}

// isChatAllowed checks if a chat ID is in the whitelist
// This is ported from Elixir's Dispatcher.filter_chat
func (d *Dispatcher) isChatAllowed(chatID int64) bool {
	// If no whitelist is configured, allow all chats
	if len(d.allowedChatIDs) == 0 {
		return true
	}
	return d.allowedChatIDs[chatID]
}

// extractCommand extracts the command name from message text
// Returns empty string if no command is found
func extractCommand(text string) string {
	if len(text) == 0 || text[0] != '/' {
		return ""
	}

	// Find the end of the command (space or end of string)
	end := len(text)
	for i, c := range text {
		if c == ' ' {
			end = i
			break
		}
	}

	// Extract command without the leading '/'
	cmd := text[1:end]

	// Handle commands with bot username (e.g., /start@mybot)
	for i, c := range cmd {
		if c == '@' {
			cmd = cmd[:i]
			break
		}
	}

	return cmd
}

// Ensure Dispatcher implements the CommandRegistrar interface
var _ CommandRegistrar = (*Dispatcher)(nil)

// CommandRegistrar is the interface for registering commands
type CommandRegistrar interface {
	Register(name string, cmd Command)
}

// SetTelegramClient sets the Telegram client for commands that need it
func (d *Dispatcher) SetTelegramClient(client telegram.Client) {
	// This method can be used to inject the client into commands that need it
	for _, cmd := range d.commands {
		if injectable, ok := cmd.(ClientInjectable); ok {
			injectable.SetClient(client)
		}
	}
}

// ClientInjectable is an interface for commands that need a Telegram client
type ClientInjectable interface {
	SetClient(client telegram.Client)
}
