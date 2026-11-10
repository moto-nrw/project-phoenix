package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// pageStore answers count entries after the offset it was asked for, of
// total in all, and notes what it was asked.
type pageStore struct {
	FinanceExportStore
	total         int
	offset, limit *int
}

func (s pageStore) slice(offset, limit int) int {
	*s.offset, *s.limit = offset, limit
	return max(min(limit, s.total-offset), 0)
}

func (s pageStore) ExportRuns(_ context.Context, _ int64, offset, limit int) (parentbilling.ExportRunPage, error) {
	return parentbilling.ExportRunPage{Runs: make([]parentbilling.ExportRun, s.slice(offset, limit)), Total: s.total}, nil
}

func (s pageStore) ExportFiles(_ context.Context, _ int64, _ string, _ int64, offset, limit int) (parentbilling.ExportProtocolPage, error) {
	return parentbilling.ExportProtocolPage{Files: make([]parentbilling.ExportProtocolEntry, s.slice(offset, limit)), Total: s.total}, nil
}

// No run and no file is cut off (#4693): every page that is not the last
// names the next one, until the list is read to its end. A fixed limit
// without a next page loses the 51st run and fails here.
func TestExportListsNameTheirNextPageUntilTheEnd(t *testing.T) {
	t.Parallel()
	size := parentbilling.ExportPageSize
	for _, total := range []int{0, 1, size, size + 1, 2*size + 7} {
		var offset, limit int
		exports := newFinanceExportsForTest(pageStore{total: total, offset: &offset, limit: &limit}, runPost{})
		read, page := 0, parentbilling.ExportPage{}
		for pages := 0; ; pages++ {
			require.Less(t, pages, 10, "total %d", total)
			runs, err := exports.ExportRuns(context.Background(), sjrReviewer, page)
			require.NoError(t, err)
			assert.Equal(t, page.Offset, offset)
			assert.Equal(t, size, limit, "one page is ExportPageSize")
			read += len(runs.Runs)
			if runs.NextOffset == nil {
				break
			}
			assert.Equal(t, page.Offset+len(runs.Runs), *runs.NextOffset)
			page = parentbilling.ExportPage{Offset: *runs.NextOffset}
		}
		assert.Equal(t, total, read, "every run of %d is on a page", total)

		read, page = 0, parentbilling.ExportPage{}
		for pages := 0; ; pages++ {
			require.Less(t, pages, 10, "total %d", total)
			files, err := exports.ExportProtocol(context.Background(), sjrReviewer, "", 0, page)
			require.NoError(t, err)
			read += len(files.Files)
			if files.NextOffset == nil {
				break
			}
			page = parentbilling.ExportPage{Offset: *files.NextOffset}
		}
		assert.Equal(t, total, read, "every file of %d is on a page", total)
	}
}
