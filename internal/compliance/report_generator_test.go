package compliance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeQuerier is an in-memory rowQuerier that recognises the report
// generator's query strings and returns pre-loaded rows. It lets us
// cover the aggregation logic without standing up Postgres or a CGO
// sqlite driver.
type fakeQuerier struct {
	rowsByQuery map[string][]fakeRow
	err         error
}

type fakeRow struct {
	Label string
	Count int64
}

// sqlRowsShim is a tiny Rows implementation that scans two columns
// (label string, count int64) from a fixed slice — the only shape the
// report generator ever reads.
type sqlRowsShim struct {
	rows   []fakeRow
	pos    int
	closed bool
}

func (s *sqlRowsShim) Next() bool {
	if s.closed || s.pos >= len(s.rows) {
		return false
	}
	return true
}
func (s *sqlRowsShim) Scan(dest ...any) error {
	if !s.Next() {
		return errors.New("sqlRowsShim: no more rows")
	}
	row := s.rows[s.pos]
	s.pos++
	// First *string gets the label, first *int64 gets the count.
	// If only one destination is passed, it must be *int64 (scalar
	// count path).
	for _, d := range dest {
		switch p := d.(type) {
		case *string:
			*p = row.Label
		case *int64:
			*p = row.Count
		}
	}
	return nil
}
func (s *sqlRowsShim) Close() error { s.closed = true; return nil }
func (s *sqlRowsShim) Err() error   { return nil }

func (f *fakeQuerier) QueryContext(_ context.Context, query string, _ ...any) (Rows, error) {
	if f.err != nil {
		return nil, f.err
	}
	rs, ok := f.rowsByQuery[query]
	if !ok {
		return nil, errors.New("fakeQuerier: unknown query: " + query)
	}
	return &sqlRowsShim{rows: rs}, nil
}

func (f *fakeQuerier) QueryRowContext(_ context.Context, _ string, _ ...any) Rows {
	return &sqlRowsShim{}
}

// --- tests -------------------------------------------------------------

func newQuerier(rowsByQuery map[string][]fakeRow) *fakeQuerier {
	return &fakeQuerier{rowsByQuery: rowsByQuery}
}

func TestReportGenerator_Audit_AggregatesByAction(t *testing.T) {
	q := newQuerier(map[string][]fakeRow{
		auditByActionQuery: {
			{Label: "login", Count: 2},
			{Label: "create_project", Count: 1},
		},
	})
	g := newReportGeneratorWithQuerier(q)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rep, err := g.Generate(ctx, ReportAudit, "2026-05")
	require.NoError(t, err)
	require.NotNil(t, rep)
	require.Equal(t, ReportAudit, rep.Type)
	require.Equal(t, "2026-05", rep.Month)
	require.Len(t, rep.Sections, 1)
	require.Equal(t, "Actions", rep.Sections[0].Title)

	counts := map[string]int64{}
	for _, row := range rep.Sections[0].Rows {
		counts[row.Label] = row.Count
	}
	require.Equal(t, int64(2), counts["login"])
	require.Equal(t, int64(1), counts["create_project"])
	require.Len(t, counts, 2)
}

func TestReportGenerator_Access_GroupsByActor(t *testing.T) {
	q := newQuerier(map[string][]fakeRow{
		accessByActorQuery: {
			{Label: "alice", Count: 2},
			{Label: "bob", Count: 1},
		},
	})
	g := newReportGeneratorWithQuerier(q)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rep, err := g.Generate(ctx, ReportAccess, "2026-05")
	require.NoError(t, err)
	require.Equal(t, ReportAccess, rep.Type)
	require.Len(t, rep.Sections, 1)
	require.Equal(t, "Logins by User", rep.Sections[0].Title)

	counts := map[string]int64{}
	for _, row := range rep.Sections[0].Rows {
		counts[row.Label] = row.Count
	}
	require.Equal(t, int64(2), counts["alice"])
	require.Equal(t, int64(1), counts["bob"])
	require.Len(t, counts, 2)
}

func TestReportGenerator_Data_SoftDeleteCounts(t *testing.T) {
	// Each scalar query returns a one-row result whose int64 column
	// is the desired count.
	q := newQuerier(map[string][]fakeRow{
		softDeletedInMonthQuery: {{Count: 1}},
		activeProjectsQuery:     {{Count: 3}},
		pendingPurgeQuery:       {{Count: 1}},
	})
	g := newReportGeneratorWithQuerier(q)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rep, err := g.Generate(ctx, ReportData, "2026-05")
	require.NoError(t, err)
	require.Equal(t, ReportData, rep.Type)
	require.Len(t, rep.Sections, 1)
	require.Equal(t, "Projects", rep.Sections[0].Title)

	counts := map[string]int64{}
	for _, row := range rep.Sections[0].Rows {
		counts[row.Label] = row.Count
	}
	require.Equal(t, int64(1), counts["soft_deleted"])
	require.Equal(t, int64(3), counts["active"])
	require.Equal(t, int64(1), counts["pending_purge"])
}

func TestReportGenerator_License_StubReturnsEmptyNoError(t *testing.T) {
	g := newReportGeneratorWithQuerier(newQuerier(nil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rep, err := g.Generate(ctx, ReportLicense, "2026-05")
	require.NoError(t, err, "license report must not error in stub form")
	require.NotNil(t, rep)
	require.Equal(t, ReportLicense, rep.Type)
	require.Equal(t, "2026-05", rep.Month)
	require.Len(t, rep.Sections, 1)
	require.Equal(t, "Dependencies", rep.Sections[0].Title)
	require.Empty(t, rep.Sections[0].Rows, "T9 will fill rows from SBOM data")
}

func TestReportGenerator_RejectsBadMonth(t *testing.T) {
	g := newReportGeneratorWithQuerier(newQuerier(nil))
	ctx := context.Background()
	_, err := g.Generate(ctx, ReportAudit, "May 2026")
	require.Error(t, err)
	_, err = g.Generate(ctx, ReportAudit, "")
	require.Error(t, err)
}

func TestReportGenerator_UnknownTypeErrors(t *testing.T) {
	g := newReportGeneratorWithQuerier(newQuerier(nil))
	_, err := g.Generate(context.Background(), ReportType("bogus"), "2026-05")
	require.Error(t, err)
}
