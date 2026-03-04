package cache

import (
	"fmt"
	"testing"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/suite"
)

type CacheDBSuite struct {
	suite.Suite
	db *testutils.TestDB
}

func (s *CacheDBSuite) SetupSuite() {
	s.db = testutils.NewTestDB(s.T())
}

func (s *CacheDBSuite) SetupTest() {
	tables := []string{"quote_entry", "quote", "cache_entry"}
	for _, table := range tables {
		s.Require().NoError(s.db.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error)
	}
}

func TestCacheDBSuite(t *testing.T) {
	suite.Run(t, new(CacheDBSuite))
}
