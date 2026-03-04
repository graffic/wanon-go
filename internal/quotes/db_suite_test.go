package quotes

import (
	"fmt"
	"testing"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/suite"
)

type QuotesDBSuite struct {
	suite.Suite
	db *testutils.TestDB
}

func (s *QuotesDBSuite) SetupSuite() {
	s.db = testutils.NewTestDB(s.T())
}

func (s *QuotesDBSuite) SetupTest() {
	tables := []string{"quote_entry", "quote", "cache_entry"}
	for _, table := range tables {
		s.Require().NoError(s.db.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error)
	}
}

func TestQuotesDBSuite(t *testing.T) {
	suite.Run(t, new(QuotesDBSuite))
}
