package adapter

import (
	"context"
	"errors"
	"testing"

	"openforge/internal/pipeline/domain"
)

// fakeOwnershipDB is a hand-rolled stand-in for the ownershipDB interface
// used by PGOwnershipRepository. It records the calls so the test can
// assert on parameters, and returns canned rows for ListByProject.
type fakeOwnershipDB struct {
	execCalls   []fakeOwnershipExecCall
	execErr     error
	execRows    int64
	listResult  []domain.ModuleOwnership
	listErr     error
	queryCalled int
}

type fakeOwnershipExecCall struct {
	query string
	args  []any
}

func (f *fakeOwnershipDB) ExecContext(ctx context.Context, query string, args ...any) (ownershipExecResult, error) {
	f.execCalls = append(f.execCalls, fakeOwnershipExecCall{query: query, args: args})
	if f.execErr != nil {
		return nil, f.execErr
	}
	return &fakeOwnershipResult{rows: f.execRows}, nil
}

func (f *fakeOwnershipDB) QueryOwnershipByProject(ctx context.Context, projectID string) ([]domain.ModuleOwnership, error) {
	f.queryCalled++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}

type fakeOwnershipResult struct {
	rows int64
}

func (r *fakeOwnershipResult) RowsAffected() (int64, error) { return r.rows, nil }

// TestPGOwnershipRepository_UpsertAndList covers the upsert + list
// roundtrip for the OwnershipRepository contract. The repository depends
// on the narrow ownershipDB interface so this test runs without a live
// PostgreSQL connection.
func TestPGOwnershipRepository_UpsertAndList(t *testing.T) {
	ctx := context.Background()
	fake := &fakeOwnershipDB{
		listResult: []domain.ModuleOwnership{
			{
				ProjectID:        "_test",
				ModuleName:       "src/foo/",
				Paths:            []string{"src/foo/"},
				TeamName:         "team-a",
				Reviewers:        []string{"u-1"},
				FallbackReviewer: "u-2",
			},
		},
	}
	repo := newOwnershipRepoFromDB(fake)

	mo := domain.ModuleOwnership{
		ProjectID:        "_test",
		ModuleName:       "src/foo/",
		Paths:            []string{"src/foo/"},
		TeamName:         "team-a",
		Reviewers:        []string{"u-1"},
		FallbackReviewer: "u-2",
	}
	if err := repo.Upsert(ctx, mo); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if len(fake.execCalls) != 1 {
		t.Fatalf("expected 1 ExecContext call, got %d", len(fake.execCalls))
	}
	if mo.ProjectID != fake.execCalls[0].args[0] {
		t.Errorf("expected first arg %q, got %q", mo.ProjectID, fake.execCalls[0].args[0])
	}

	// Idempotent upsert (2nd call) must not error.
	if err := repo.Upsert(ctx, mo); err != nil {
		t.Fatalf("Upsert (2nd call) returned error: %v", err)
	}
	if len(fake.execCalls) != 2 {
		t.Fatalf("expected 2 ExecContext calls after 2nd upsert, got %d", len(fake.execCalls))
	}

	list, err := repo.ListByProject(ctx, "_test")
	if err != nil {
		t.Fatalf("ListByProject returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 ownership record, got %d", len(list))
	}
	if list[0].ProjectID != "_test" {
		t.Errorf("expected ProjectID=_test, got %q", list[0].ProjectID)
	}
	if list[0].ModuleName != "src/foo/" {
		t.Errorf("expected ModuleName=src/foo/, got %q", list[0].ModuleName)
	}
}

// TestPGOwnershipRepository_UpsertError verifies the repository surfaces
// underlying DB errors instead of swallowing them.
func TestPGOwnershipRepository_UpsertError(t *testing.T) {
	ctx := context.Background()
	fake := &fakeOwnershipDB{execErr: errors.New("db unreachable")}
	repo := newOwnershipRepoFromDB(fake)

	err := repo.Upsert(ctx, domain.ModuleOwnership{
		ProjectID:  "_x",
		ModuleName: "src/x/",
		Paths:      []string{"src/x/"},
		TeamName:   "team",
		Reviewers:  []string{"u"},
	})
	if err == nil {
		t.Fatal("expected error from Upsert, got nil")
	}
}

// TestPGOwnershipRepository_ListError verifies ListByProject propagates
// errors from the underlying DB.
func TestPGOwnershipRepository_ListError(t *testing.T) {
	ctx := context.Background()
	fake := &fakeOwnershipDB{listErr: errors.New("query failed")}
	repo := newOwnershipRepoFromDB(fake)

	_, err := repo.ListByProject(ctx, "_test")
	if err == nil {
		t.Fatal("expected error from ListByProject, got nil")
	}
}
