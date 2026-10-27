package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// These edges preserve atomic database invariants, not permissions for a Go
// adapter to write another owner's table. The two meta tables are migration
// rollback provenance (1.15.363/365), not runtime records owned by a domain.
var crossOwnerTriggerWrites = map[string]string{
	"schedule.instance_staff:capture_instance_staff_feed_tombstone:calendar.staff_feed_tombstones":         "An ICS subscription needs a cancellation when its staff assignment is removed; recording the tombstone must commit with the assignment change (1.15.342).",
	"schedule.activity_instances:capture_activity_instance_feed_tombstones:calendar.staff_feed_tombstones": "An ICS subscription needs a cancellation when its timetable occurrence is removed; the tombstone is atomic with occurrence deletion (1.15.342).",
	"schedule.staff_shifts:capture_staff_shift_feed_tombstone:calendar.staff_feed_tombstones":              "An ICS subscription needs a cancellation when a shift is removed or reassigned; the tombstone is atomic with the shift mutation (1.15.342).",

	"active.student_status_days:record_meal_sickness_status:schedule.meal_sickness_status_history":                                  "Record the meal-count snapshot in the same commit as a sickness change; an application callback cannot preserve trigger semantics for every writer.",
	"active.student_status_days:record_deleted_meal_sickness_status:schedule.meal_sickness_status_history":                          "Record a removed sickness in the same commit, including the retention purge's deliberate history suppression (1.15.446).",
	"auth.guardian_student_access:invalidate_meal_participation_permission_grant:meta.meal_participation_permission_grants":         "Invalidate migration rollback provenance at the exact permission change; otherwise Down could restore a withdrawn meal permission.",
	"auth.guardian_student_access:invalidate_parent_student_consent_permission_grant:meta.parent_student_consent_permission_grants": "Invalidate migration rollback provenance at the exact permission change; otherwise Down could restore a withdrawn consent permission.",
	"users.guardian_profiles:guardian_profiles_bind_student_access:auth.guardian_student_access":                                    "Keep the portal account binding consistent with profile linkage/unlinkage for every SQL writer, atomically with the profile update.",
}

// Evaluate the installed schema, not historical Up/Down text or a hand-written
// list of trigger names. Qualified routine calls are followed transitively;
// dynamic SQL and unresolved write targets fail closed.
func TestDatabaseTriggerOwnership(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	root, err := findBackendRoot()
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(root, "architecture/policy.json"))
	require.NoError(t, err)
	var objects struct {
		DataObjects []struct {
			Name  string `json:"name"`
			Owner string `json:"write_owner"`
		} `json:"data_objects"`
	}
	require.NoError(t, json.Unmarshal(raw, &objects))
	owners := map[string]string{}
	for _, object := range objects.DataObjects {
		owners[object.Name] = object.Owner
	}
	// These ledgers retain only which grants a migration introduced, for Down.
	owners["meta.parent_student_consent_permission_grants"] = "migrations"
	owners["meta.meal_participation_permission_grants"] = "migrations"
	var routines []struct {
		ID         int64
		Name, Body string
	}
	require.NoError(t, db.NewRaw(`SELECT p.oid AS id, n.nspname || '.' || p.proname AS name, p.prosrc AS body
  FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname NOT IN ('pg_catalog','information_schema')`).Scan(Ctx(t), &routines))
	bodies := map[int64]string{}
	byName := map[string][]int64{}
	for _, routine := range routines {
		bodies[routine.ID] = routine.Body
		byName[routine.Name] = append(byName[routine.Name], routine.ID)
	}
	var triggers []struct {
		Name, Source string
		FunctionID   int64
	}
	require.NoError(t, db.NewRaw(`SELECT t.tgname AS name, n.nspname || '.' || c.relname AS source, t.tgfoid AS function_id
  FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE NOT t.tgisinternal AND n.nspname NOT IN ('pg_catalog','information_schema')`).Scan(Ctx(t), &triggers))
	seen := map[string]bool{}
	for _, trigger := range triggers {
		writes, err := triggerRoutineWrites(trigger.FunctionID, bodies, byName, map[int64]bool{})
		require.NoError(t, err, trigger.Source+":"+trigger.Name)
		for target := range writes {
			require.NotEmpty(t, owners[trigger.Source], "unclassified trigger source: %s", trigger.Source)
			require.NotEmpty(t, owners[target], "unclassified trigger write: %s -> %s", trigger.Source, target)
			if owners[trigger.Source] == owners[target] {
				continue
			}
			key := trigger.Source + ":" + trigger.Name + ":" + target
			require.NotEmpty(t, crossOwnerTriggerWrites[key], "unreviewed cross-owner trigger write: %s (%s -> %s)", key, owners[trigger.Source], owners[target])
			seen[key] = true
		}
	}
	for key := range crossOwnerTriggerWrites {
		require.True(t, seen[key], "stale trigger edge, remove: %s", key)
	}
}

func triggerRoutineWrites(id int64, bodies map[int64]string, byName map[string][]int64, visited map[int64]bool) (map[string]bool, error) {
	result := map[string]bool{}
	if visited[id] {
		return result, nil
	}
	visited[id] = true
	body, ok := bodies[id]
	if !ok {
		return nil, fmt.Errorf("missing routine %d", id)
	}
	tokens := triggerSQLTokens(body)
	for i, word := range tokens {
		upper := strings.ToUpper(word)
		if upper == "EXECUTE" {
			return nil, fmt.Errorf("dynamic SQL in trigger routine %d must be resolved", id)
		}
		target := -1
		if upper == "INSERT" && i+1 < len(tokens) && strings.EqualFold(tokens[i+1], "INTO") {
			target = i + 2
		}
		if upper == "DELETE" && i+1 < len(tokens) && strings.EqualFold(tokens[i+1], "FROM") {
			target = i + 2
		}
		if upper == "UPDATE" && !triggerLockClause(tokens, i) && (i == 0 || !strings.EqualFold(tokens[i-1], "DO")) {
			target = i + 1
		}
		if target >= 0 {
			if target+2 >= len(tokens) || tokens[target+1] != "." {
				return nil, fmt.Errorf("unqualified/unresolved trigger write in routine %d near %s", id, word)
			}
			result[tokens[target]+"."+tokens[target+2]] = true
		}
		if i+3 < len(tokens) && tokens[i+1] == "." && tokens[i+3] == "(" {
			for _, callee := range byName[word+"."+tokens[i+2]] {
				writes, err := triggerRoutineWrites(callee, bodies, byName, visited)
				if err != nil {
					return nil, err
				}
				for table := range writes {
					result[table] = true
				}
			}
		}
	}
	return result, nil
}

// triggerLockClause reports whether the UPDATE at i ends a row-locking
// clause (FOR UPDATE, FOR NO KEY UPDATE), which writes no table.
func triggerLockClause(tokens []string, i int) bool {
	if i >= 1 && strings.EqualFold(tokens[i-1], "FOR") {
		return true
	}
	return i >= 3 && strings.EqualFold(tokens[i-1], "KEY") && strings.EqualFold(tokens[i-2], "NO") && strings.EqualFold(tokens[i-3], "FOR")
}

// SQL quotes/comments are removed before recognizing statements. PostgreSQL
// double-quoted identifiers are kept; single-quoted text is never a table.
func triggerSQLTokens(source string) []string {
	var cleaned strings.Builder
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "--") {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				break
			}
			i += end
			continue
		}
		if strings.HasPrefix(source[i:], "/*") {
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 4
			continue
		}
		if source[i] == '\'' {
			i++
			for i < len(source) {
				if source[i] == '\'' {
					i++
					if i >= len(source) || source[i] != '\'' {
						break
					}
				}
				i++
			}
			cleaned.WriteByte(' ')
			continue
		}
		if source[i] == '"' {
			i++
			for i < len(source) {
				if source[i] == '"' {
					i++
					if i >= len(source) || source[i] != '"' {
						break
					}
				}
				cleaned.WriteByte(source[i])
				i++
			}
			continue
		}
		cleaned.WriteByte(source[i])
		i++
	}
	return regexp.MustCompile(`[\pL\pN_$]+|[().;]`).FindAllString(cleaned.String(), -1)
}

func TestTriggerOwnershipFollowsFunctionsAndRejectsDynamicWrites(t *testing.T) {
	t.Parallel()
	bodies := map[int64]string{1: `BEGIN PERFORM owner.child(); RETURN NEW; END`, 2: `BEGIN INSERT INTO other.rows(id) VALUES(NEW.id); RETURN NEW; END`}
	writes, err := triggerRoutineWrites(1, bodies, map[string][]int64{"owner.child": {2}}, map[int64]bool{})
	require.NoError(t, err)
	var tables []string
	for table := range writes {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	require.Equal(t, []string{"other.rows"}, tables)
	bodies[2] = `BEGIN EXECUTE 'UPDATE other.rows SET id = 1'; END`
	_, err = triggerRoutineWrites(1, bodies, map[string][]int64{"owner.child": {2}}, map[int64]bool{})
	require.ErrorContains(t, err, "dynamic SQL")
	bodies[2] = `BEGIN UPDATE rows SET id = 1; END`
	_, err = triggerRoutineWrites(1, bodies, map[string][]int64{"owner.child": {2}}, map[int64]bool{})
	require.ErrorContains(t, err, "unqualified/unresolved")
	bodies[2] = `BEGIN PERFORM 1 FROM owner.rows ORDER BY id FOR NO KEY UPDATE; PERFORM 1 FROM owner.rows FOR UPDATE; UPDATE other.rows SET id = 1; END`
	writes, err = triggerRoutineWrites(1, bodies, map[string][]int64{"owner.child": {2}}, map[int64]bool{})
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"other.rows": true}, writes, "a row-locking clause writes no table")
}
