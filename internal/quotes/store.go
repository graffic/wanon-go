package quotes

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Store handles persistence of quotes to the database
type Store struct {
	db *gorm.DB
}

// NewStore creates a new quote store
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// StoreOptions contains options for storing a quote
type StoreOptions struct {
	Creator   map[string]interface{} // Telegram User who created the quote
	ChatID    int64
	Entries   []CacheEntry           // Cache entries to store as quote entries
}

// Store saves a quote with its entries to the database.
// This ports the Quotes.Store.store functionality from Elixir.
// Entries are stored with correct order (0, 1, 2...).
func (s *Store) Store(ctx context.Context, opts StoreOptions) (*Quote, error) {
	if len(opts.Entries) == 0 {
		return nil, fmt.Errorf("cannot store quote with no entries")
	}

	// Convert creator to JSON
	creatorJSON, err := json.Marshal(opts.Creator)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal creator: %w", err)
	}

	// Create quote within a transaction
	var quote Quote
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the quote
		quote = Quote{
			Creator: creatorJSON,
			ChatID:  opts.ChatID,
		}
		if err := tx.Create(&quote).Error; err != nil {
			return fmt.Errorf("failed to create quote: %w", err)
		}

		// Create quote entries with correct order (0, 1, 2...)
		for i, entry := range opts.Entries {
			quoteEntry := QuoteEntry{
				Order:   i, // Order starts at 0
				Message: entry.Message,
				QuoteID: quote.ID,
			}
			if err := tx.Create(&quoteEntry).Error; err != nil {
				return fmt.Errorf("failed to create quote entry at order %d: %w", i, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Reload quote with entries
	if err := s.db.WithContext(ctx).
		Preload("Entries").
		First(&quote, quote.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload quote with entries: %w", err)
	}

	return &quote, nil
}

// StoreFromBuild stores a quote from a build result
func (s *Store) StoreFromBuild(ctx context.Context, creator map[string]interface{}, result *BuildResult) (*Quote, error) {
	return s.Store(ctx, StoreOptions{
		Creator: creator,
		ChatID:  result.ChatID,
		Entries: result.Entries,
	})
}

// GetByID retrieves a quote by its ID, including all entries
func (s *Store) GetByID(ctx context.Context, id uint) (*Quote, error) {
	var quote Quote
	if err := s.db.WithContext(ctx).
		Preload("Entries", func(db *gorm.DB) *gorm.DB {
			return db.Order("quote_entries.order ASC")
		}).
		First(&quote, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}
	return &quote, nil
}

// GetRandomForChat retrieves a random quote for a specific chat
func (s *Store) GetRandomForChat(ctx context.Context, chatID int64) (*Quote, error) {
	var quote Quote
	
	// Use random ordering - PostgreSQL specific
	err := s.db.WithContext(ctx).
		Where("chat_id = ?", chatID).
		Order("RANDOM()").
		Preload("Entries", func(db *gorm.DB) *gorm.DB {
			return db.Order("quote_entries.order ASC")
		}).
		First(&quote).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No quotes found
		}
		return nil, fmt.Errorf("failed to get random quote: %w", err)
	}
	
	return &quote, nil
}

// CountForChat returns the number of quotes in a chat
func (s *Store) CountForChat(ctx context.Context, chatID int64) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&Quote{}).
		Where("chat_id = ?", chatID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count quotes: %w", err)
	}
	return count, nil
}

// Delete deletes a quote and its entries (cascade delete handled by GORM constraint)
func (s *Store) Delete(ctx context.Context, id uint) error {
	if err := s.db.WithContext(ctx).Delete(&Quote{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete quote: %w", err)
	}
	return nil
}

// Helper function to convert map to datatypes.JSON
func MapToJSON(m map[string]interface{}) (datatypes.JSON, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return data, nil
}
