package migrations

import "testing"

func TestCatalogIsOrderedUniqueAndOwned(t *testing.T) {
	items, err := Compose(PlatformCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("catalog must contain the platform migration")
	}
	var prior int64
	for _, item := range items {
		if item.Version <= prior || item.Owner == "" || item.Name == "" || item.SQL == "" {
			t.Fatalf("invalid migration: %+v", item)
		}
		prior = item.Version
	}
}

func TestComposeAcceptsFutureModuleWithoutPlatformImportingIt(t *testing.T) {
	futureModule := []Migration{New(20, "future", "create_projection", "CREATE SCHEMA future")}
	items, err := Compose(futureModule, PlatformCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Owner != "platform" || items[1].Owner != "future" {
		t.Fatalf("unexpected composed catalog: %+v", items)
	}
}

func TestComposeRejectsDuplicateVersions(t *testing.T) {
	_, err := Compose(
		[]Migration{New(2, "one", "first", "SELECT 1")},
		[]Migration{New(2, "two", "second", "SELECT 2")},
	)
	if err == nil {
		t.Fatal("expected duplicate version rejection")
	}
}
