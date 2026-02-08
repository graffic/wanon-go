package quotes

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Quote represents a saved quote in the database (ported from Elixir Quote schema)
type Quote struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Creator   datatypes.JSON `gorm:"type:jsonb;not null" json:"creator"` // Telegram User who created the quote
	ChatID    int64          `gorm:"index;not null" json:"chat_id"`
	CreatedAt time.Time      `json:"created_at"`

	// Associations - entries are ordered by the Order field in QuoteEntry
	Entries []QuoteEntry `gorm:"foreignKey:QuoteID;constraint:OnDelete:CASCADE;" json:"entries,omitempty"`
}

// TableName specifies the table name for Quote
func (Quote) TableName() string {
	return "quote"
}

// QuoteEntry represents a single message entry within a quote (ported from Elixir QuoteEntry schema)
type QuoteEntry struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Order     int            `gorm:"not null" json:"order"`              // Order in the quote thread (0, 1, 2...)
	Message   datatypes.JSON `gorm:"type:jsonb;not null" json:"message"` // Full Telegram message as JSON
	QuoteID   uint           `gorm:"index;not null" json:"quote_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name for QuoteEntry
func (QuoteEntry) TableName() string {
	return "quote_entry"
}
