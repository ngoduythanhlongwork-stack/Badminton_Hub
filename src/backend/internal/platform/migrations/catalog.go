package migrations

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"sort"
)

//go:embed sql/*.sql
var platformSQL embed.FS

// Migration is an immutable, module-owned schema change. Versions are global and
// monotonically increasing so deploy order is deterministic across modules.
type Migration struct {
	Version int64
	Owner   string
	Name    string
	SQL     string
	Hash    [sha256.Size]byte
}

// New creates a module-owned migration descriptor. A module may expose a slice
// of these from its public package; the executable supplies those slices to the
// runner, so platform never imports a business module.
func New(version int64, owner, name, sql string) Migration {
	return Migration{Version: version, Owner: owner, Name: name, SQL: sql, Hash: sha256.Sum256([]byte(sql))}
}

// PlatformCatalog contains technical migrations owned by platform only.
func PlatformCatalog() []Migration {
	contents, err := platformSQL.ReadFile("sql/0001_platform.sql")
	if err != nil {
		panic(err)
	}
	return []Migration{New(1, "platform", "outbox", string(contents))}
}

// Compose produces the globally ordered deploy manifest from independently
// owned groups and rejects ambiguous or incomplete metadata.
func Compose(groups ...[]Migration) ([]Migration, error) {
	var items []Migration
	for _, group := range groups {
		items = append(items, group...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Version < items[j].Version })
	versions := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.Version <= 0 || item.Owner == "" || item.Name == "" || item.SQL == "" {
			return nil, fmt.Errorf("migration metadata must include positive version, owner, name, and SQL")
		}
		if _, exists := versions[item.Version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d", item.Version)
		}
		versions[item.Version] = struct{}{}
	}
	return items, nil
}
