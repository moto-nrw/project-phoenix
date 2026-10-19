package test

import (
	"encoding/json"
	"os"
	"testing"
)

func TestDumpTransactionInventory4088(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	sites, err := transactionSites(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("/tmp/4088-transaction-sites.json", raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestDumpTriggerInventory4088(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	var rows []struct {
		Name   string
		Source string
		Body   string
	}
	err := db.NewRaw(`SELECT t.tgname AS name, n.nspname || '.' || c.relname AS source, pg_get_functiondef(t.tgfoid) AS body FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE NOT t.tgisinternal AND n.nspname NOT IN ('pg_catalog','information_schema') ORDER BY source,name`).Scan(Ctx(t), &rows)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("/tmp/4088-trigger-sites.json", raw, 0600); err != nil {
		t.Fatal(err)
	}
}
