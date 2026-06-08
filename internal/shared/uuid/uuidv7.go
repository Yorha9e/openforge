// Package uuid is the project's single entry point for generating UUID v7
// strings. All call sites that mint a new identifier must use New() so the
// format stays consistent with the PostgreSQL uuid_generate_v7() function
// installed by migrations/015_uuid_v7_extension.up.sql.
package uuid

import "github.com/google/uuid"

// New returns a freshly minted UUID v7 in canonical 8-4-4-4-12 hex form.
//
// RFC 9562 v7 packs a 48-bit unix-ms timestamp into the first 6 bytes,
// giving B-tree friendly monotonic ordering. This matches the
// uuid_generate_v7() SQL function so application- and DB-side IDs are
// indistinguishable.
func New() string {
	return uuid.Must(uuid.NewV7()).String()
}
