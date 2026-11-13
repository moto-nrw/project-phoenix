package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

// TraegerSettingsRuntime opens the Träger transaction (ADR 0086 section 5)
// and takes transaction-scoped advisory locks in it.
type TraegerSettingsRuntime interface {
	WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error
	AcquireLock(ctx context.Context, key string, shared bool) error
}

// TraegerSettingsDependencies are the collaborators of the Träger level,
// fixed at construction. Rights maps each Träger write right of a
// definition to the right name the session carries; it must name all three.
type TraegerSettingsDependencies struct {
	Values   config.TraegerSettingRepository
	Runtime  TraegerSettingsRuntime
	Schools  func(ctx context.Context, organizationID int64) ([]int64, error)
	Notify   config.SettingsChangedNotifier
	Rights   map[config.TraegerRight]string
	Logger   *slog.Logger
	Now      func() time.Time
	Registry *config.Registry
}

// TraegerSettings is the Träger-Büro's side of the settings system: the
// Träger values of every Träger-level definition, read and written in the
// Träger transaction of the session's Träger.
type TraegerSettings struct {
	deps TraegerSettingsDependencies
}

// NewTraegerSettings builds the Träger level. A missing collaborator or an
// unmapped Träger right is a construction error.
func NewTraegerSettings(deps TraegerSettingsDependencies) (*TraegerSettings, error) {
	if deps.Values == nil || deps.Runtime == nil || deps.Schools == nil || deps.Notify == nil || deps.Logger == nil || deps.Now == nil {
		return nil, errors.New("traeger settings: all dependencies are required")
	}
	for _, right := range []config.TraegerRight{config.TraegerRightSettings, config.TraegerRightStaff, config.TraegerRightBilling} {
		if deps.Rights[right] == "" {
			return nil, fmt.Errorf("traeger settings: Träger right %q is not mapped", right)
		}
	}
	if deps.Registry == nil {
		deps.Registry = config.DefaultRegistry()
	}
	deps.Logger = deps.Logger.With("service", "traeger_settings")
	return &TraegerSettings{deps: deps}, nil
}

// traegerKeys lists the definitions that may hold a Träger value.
func (t *TraegerSettings) traegerKeys() map[string]*config.Definition {
	result := map[string]*config.Definition{}
	for key, definition := range t.deps.Registry.AllDefinitions() {
		if definition.Levels.AtTraeger() {
			result[key] = definition
		}
	}
	return result
}

// Schema returns every Träger-level setting with the Träger's effective value
// (its own value or the registry default) and where it comes from. rights are
// the Träger rights of the session; they decide which items are writable.
func (t *TraegerSettings) Schema(ctx context.Context, organizationID int64, rights []string) (any, error) {
	definitions := t.traegerKeys()
	keys := make([]string, 0, len(definitions))
	for key := range definitions {
		keys = append(keys, key)
	}
	var stored []*config.TraegerSettingValue
	err := t.deps.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var findErr error
		stored, findErr = t.deps.Values.FindByOrganization(txCtx, organizationID, keys)
		return findErr
	})
	if err != nil {
		return nil, &config.SettingsError{Op: "traeger_schema", Err: err}
	}
	snapshot, err := config.NewLayeredSettingsSnapshot(t.deps.Registry, 0, keys, nil, stored)
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]*config.ResolvedSetting, len(definitions))
	for key, definition := range definitions {
		item, itemErr := t.resolve(key, definition, snapshot, rights)
		if itemErr != nil {
			return nil, itemErr
		}
		resolved[key] = item
	}
	memo := make(map[string]bool, len(resolved))
	for _, item := range resolved {
		if item.DependsOn != nil && resolved[item.DependsOn.Key] == nil {
			// The parent is a school setting the Träger does not see; the
			// school decides whether the dependent value matters.
			memo[item.Key] = true
		}
	}
	for _, item := range resolved {
		item.Visible = evaluateDependency(item, resolved, memo, map[string]bool{})
	}
	return groupSettingsSchema(t.deps.Registry, resolved), nil
}

// TraegerTexts reads the effective values of Träger-level text settings
// of a Träger: its own value or the registry default. Documents of the
// Träger print them (ADR 0112, #4669). It joins the caller's Träger
// transaction; a key without a Träger level is an error.
func (t *TraegerSettings) TraegerTexts(ctx context.Context, organizationID int64, keys []string) (map[string]string, error) {
	snapshot, err := t.traegerSnapshot(ctx, organizationID, keys, "traeger_texts")
	if err != nil {
		return nil, err
	}
	texts := make(map[string]string, len(keys))
	for _, key := range keys {
		if texts[key], err = snapshot.String(key); err != nil {
			return nil, err
		}
	}
	return texts, nil
}

// TraegerBools reads the effective values of Träger-level switches of a
// Träger: its own value or the registry default. The Elternabrechnung asks
// whether the Träger enters the debtor number when it confirms a holiday
// registration (#5621). It joins the caller's Träger transaction; a key
// without a Träger level is an error.
func (t *TraegerSettings) TraegerBools(ctx context.Context, organizationID int64, keys []string) (map[string]bool, error) {
	snapshot, err := t.traegerSnapshot(ctx, organizationID, keys, "traeger_bools")
	if err != nil {
		return nil, err
	}
	values := make(map[string]bool, len(keys))
	for _, key := range keys {
		if values[key], err = snapshot.Bool(key); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// TraegerInts reads the effective values of Träger-level whole numbers of a
// Träger: its own value or the registry default. The Träger dashboard reads
// the thresholds of its hints (#4675), the Betreuungsvertrag after how many
// days it reminds parents of an unsigned contract (#4679). It opens the
// Träger transaction; a key without a Träger level, or a value that is no
// whole number, is an
// error.
func (t *TraegerSettings) TraegerInts(ctx context.Context, organizationID int64, keys []string) (map[string]int, error) {
	snapshot, err := t.traegerSnapshot(ctx, organizationID, keys, "traeger_ints")
	if err != nil {
		return nil, err
	}
	values := make(map[string]int, len(keys))
	for _, key := range keys {
		if values[key], err = snapshot.Int(key); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// traegerSnapshot reads the Träger values of keys in the caller's Träger
// transaction, layered over the registry defaults.
func (t *TraegerSettings) traegerSnapshot(ctx context.Context, organizationID int64, keys []string, operation string) (*config.SettingsSnapshot, error) {
	definitions := t.traegerKeys()
	for _, key := range keys {
		if definitions[key] == nil {
			return nil, fmt.Errorf("traeger settings: %q has no Träger level", key)
		}
	}
	var stored []*config.TraegerSettingValue
	err := t.deps.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var findErr error
		stored, findErr = t.deps.Values.FindByOrganization(txCtx, organizationID, keys)
		return findErr
	})
	if err != nil {
		return nil, &config.SettingsError{Op: operation, Err: err}
	}
	return config.NewLayeredSettingsSnapshot(t.deps.Registry, 0, keys, nil, stored)
}

func (t *TraegerSettings) resolve(key string, definition *config.Definition, snapshot *config.SettingsSnapshot, rights []string) (*config.ResolvedSetting, error) {
	value, err := snapshot.Value(key)
	if err != nil {
		return nil, err
	}
	source, _, changedAt, err := snapshot.Layer(key)
	if err != nil {
		return nil, err
	}
	return &config.ResolvedSetting{
		Key: key, Label: definition.Label, Description: definition.Description, Placeholder: definition.Placeholder,
		Type:    definition.Type,
		Default: definition.Default, Value: value, IsDefault: source == config.SourceDefault,
		Writable: slices.Contains(rights, t.deps.Rights[definition.TraegerWriteRight]), Visible: true,
		SortOrder: definition.SortOrder, AccessPolicy: definition.AccessPolicy, Levels: definition.Levels,
		Source: source, TraegerChangedAt: changedAt, Validation: definition.Validation,
		DependsOn: definition.DependsOn, Options: definition.Options,
	}, nil
}

// writeDefinition returns the Träger-level definition of key after checking
// the session's right for it.
func (t *TraegerSettings) writeDefinition(key string, rights []string, operation string) (*config.Definition, error) {
	definition := t.deps.Registry.GetDefinition(key)
	if definition == nil || !definition.Levels.AtTraeger() {
		return nil, &config.SettingsError{Op: operation, Err: &config.DefinitionNotFoundError{Key: key}}
	}
	required := t.deps.Rights[definition.TraegerWriteRight]
	if !slices.Contains(rights, required) {
		return nil, &config.SettingsError{Op: operation, Err: &config.PermissionDeniedError{Key: key, RequiredPermission: required}}
	}
	return definition, nil
}

// SetValue stores the Träger's value of key for all its schools. The change
// and its entry in the change log commit together; a failed entry fails the
// change (ADR 0086 section 7). It opens its own Träger transaction and
// rejects an ambient one of another kind.
func (t *TraegerSettings) SetValue(ctx context.Context, organizationID, accountID int64, rights []string, key string, value any) error {
	return t.setValue(ctx, organizationID, traegerActor{account: &accountID}, rights, key, value)
}

// ResetValue removes the Träger's value of key: its schools return to the
// registry default unless they keep an override of their own.
func (t *TraegerSettings) ResetValue(ctx context.Context, organizationID, accountID int64, rights []string, key string) error {
	return t.resetValue(ctx, organizationID, traegerActor{account: &accountID}, rights, key)
}

// operatorRights are the Träger rights of an operator: every right a
// definition can name. The operator dashboard thereby shows and writes the
// same Träger-level keys as the Träger-Büro with every right; the registry
// and its write rights stay the source (#5748).
func (t *TraegerSettings) operatorRights() []string {
	rights := make([]string, 0, len(t.deps.Rights))
	for _, right := range t.deps.Rights {
		rights = append(rights, right)
	}
	return rights
}

// OperatorSchema returns the Träger settings of organizationID for the
// operator dashboard (#5748): the schema of the Träger-Portal, every item
// writable.
func (t *TraegerSettings) OperatorSchema(ctx context.Context, organizationID int64) (any, error) {
	return t.Schema(ctx, organizationID, t.operatorRights())
}

// OperatorSetValue stores a Träger value an operator set. beforeChange runs
// after the key and the value passed the registry's checks and before the
// Träger transaction opens; the operator dashboard records its operator audit
// there, so a refused value leaves no entry and no change happens without
// one. The change log names the operator in changed_by_operator; the value
// keeps no account.
func (t *TraegerSettings) OperatorSetValue(ctx context.Context, organizationID, operatorID int64, key string, value any, beforeChange func(context.Context) error) error {
	if operatorID <= 0 || beforeChange == nil {
		return &config.SettingsError{Op: "traeger_set_value", Err: errors.New("operator and audit are required")}
	}
	return t.setValue(ctx, organizationID, traegerActor{operator: &operatorID, beforeChange: beforeChange}, t.operatorRights(), key, value)
}

// OperatorResetValue removes a Träger value on behalf of an operator, with
// beforeChange as in OperatorSetValue.
func (t *TraegerSettings) OperatorResetValue(ctx context.Context, organizationID, operatorID int64, key string, beforeChange func(context.Context) error) error {
	if operatorID <= 0 || beforeChange == nil {
		return &config.SettingsError{Op: "traeger_reset_value", Err: errors.New("operator and audit are required")}
	}
	return t.resetValue(ctx, organizationID, traegerActor{operator: &operatorID, beforeChange: beforeChange}, t.operatorRights(), key)
}

// traegerActor is who changes a Träger value: an account of the Träger-Büro
// or an operator, never both. beforeChange, set for an operator, runs after
// the checks and before the Träger transaction.
type traegerActor struct {
	account      *int64
	operator     *int64
	beforeChange func(context.Context) error
}

func (t *TraegerSettings) setValue(ctx context.Context, organizationID int64, actor traegerActor, rights []string, key string, value any) error {
	definition, err := t.writeDefinition(key, rights, "traeger_set_value")
	if err != nil {
		return err
	}
	if err := validateValue(definition, value); err != nil {
		return &config.SettingsError{Op: "traeger_set_value", Err: &config.InvalidValueError{Key: key, Reason: err.Error()}}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return &config.SettingsError{Op: "traeger_set_value", Err: fmt.Errorf("marshal value: %w", err)}
	}
	return t.change(ctx, organizationID, actor, key, "set", encoded)
}

func (t *TraegerSettings) resetValue(ctx context.Context, organizationID int64, actor traegerActor, rights []string, key string) error {
	if _, err := t.writeDefinition(key, rights, "traeger_reset_value"); err != nil {
		return err
	}
	return t.change(ctx, organizationID, actor, key, "reset", nil)
}

func (t *TraegerSettings) change(ctx context.Context, organizationID int64, actor traegerActor, key, action string, value json.RawMessage) error {
	if actor.beforeChange != nil {
		if err := actor.beforeChange(ctx); err != nil {
			return &config.SettingsError{Op: "traeger_" + action + "_value", Err: err}
		}
	}
	err := t.deps.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		return t.writeChange(txCtx, organizationID, actor, key, action, value)
	})
	if err != nil {
		return &config.SettingsError{Op: "traeger_" + action + "_value", Err: err}
	}
	t.deps.Logger.Info("Träger setting changed",
		"key", key,
		"organization_id", organizationID,
		"action", action,
		"by_operator", actor.operator != nil,
	)
	// The command owns its Träger transaction, so it has committed here.
	t.notifySchools(ctx, organizationID, key)
	return nil
}

// writeChange writes the Träger value of key and its change-log entry in the
// caller's Träger transaction.
func (t *TraegerSettings) writeChange(txCtx context.Context, organizationID int64, actor traegerActor, key, action string, value json.RawMessage) error {
	// The row lock below cannot hold a value that does not exist yet: two
	// first writes, or a first write and a reset, would both log no old
	// value. The advisory lock orders every change of this Träger and key.
	if err := t.deps.Runtime.AcquireLock(txCtx, traegerSettingLockKey(organizationID, key), false); err != nil {
		return err
	}
	existing, err := t.deps.Values.LockByOrganizationAndKey(txCtx, organizationID, key)
	if err != nil {
		return err
	}
	if action == "set" {
		err = t.deps.Values.Upsert(txCtx, &config.TraegerSettingValue{OrganizationID: organizationID, SettingKey: key, Value: value, UpdatedBy: actor.account})
	} else {
		err = t.deps.Values.Delete(txCtx, organizationID, key)
	}
	if err != nil {
		return err
	}
	var old json.RawMessage
	if existing != nil {
		old = existing.Value
	}
	now := t.deps.Now()
	entry := &config.TraegerSettingAuditEntry{
		OrganizationID: organizationID, SettingKey: key, OldValue: old, NewValue: value,
		Action: action, ChangedBy: actor.account, ChangedByOperator: actor.operator, ChangedAt: now,
	}
	// Retention: changes are kept as long as the school's setting
	// changes (three years); each change of the Träger drops older ones.
	if err := t.deps.Values.RecordChange(txCtx, entry); err != nil {
		return err
	}
	return nil
}

// traegerSettingLockKey names the advisory lock of one Träger value.
func traegerSettingLockKey(organizationID int64, key string) string {
	return fmt.Sprintf("traeger-setting:%d:%s", organizationID, key)
}

// ClassifyError names the HTTP class of a Träger settings error.
func (*TraegerSettings) ClassifyError(err error) string {
	switch {
	case errors.Is(err, config.ErrPermissionDenied):
		return "forbidden"
	case errors.Is(err, config.ErrDefinitionNotFound):
		return "not_found"
	case errors.Is(err, config.ErrInvalidValue):
		return "invalid"
	}
	return "internal"
}

// notifySchools sends tenant_settings_changed to every school of the Träger
// after the commit (ADR 0086 section 8). The notice is lossy: a failed school
// lookup is logged, and open tabs still re-read on their next request.
func (t *TraegerSettings) notifySchools(ctx context.Context, organizationID int64, key string) {
	schools, err := t.deps.Schools(context.WithoutCancel(ctx), organizationID)
	if err != nil {
		t.deps.Logger.Warn("tenant_settings_changed for the Träger schools skipped",
			"organization_id", organizationID,
			"error", err.Error(),
		)
		return
	}
	for _, schoolID := range schools {
		t.deps.Notify(ctx, schoolID, key)
	}
}
