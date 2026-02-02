package bot

import (
	"context"

	"github.com/go-telegram/bot/models"
)

// Command represents a bot command that can be executed
type Command interface {
	// Execute runs the command with the given message context
	Execute(ctx context.Context, msg *models.Message) error
}

// CommandFunc is an adapter to allow ordinary functions to be used as commands
type CommandFunc func(ctx context.Context, msg *models.Message) error

// Execute implements the Command interface
func (f CommandFunc) Execute(ctx context.Context, msg *models.Message) error {
	return f(ctx, msg)
}

// Registry holds all registered commands
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a new command registry
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

// Register adds a command to the registry
func (r *Registry) Register(name string, cmd Command) {
	r.commands[name] = cmd
}

// Get retrieves a command by name
func (r *Registry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Has checks if a command is registered
func (r *Registry) Has(name string) bool {
	_, ok := r.commands[name]
	return ok
}

// List returns all registered command names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}
