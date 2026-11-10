package config

import (
	"encoding/json"
	"errors"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// settingExportColumns names the value columns of each setting table the
// return export projects (#3838). Every table keys its values by setting_key.
var settingExportColumns = map[string]map[string]bool{
	"config.setting_values":         {"value": true},
	"config.traeger_setting_values": {"value": true},
	"config.setting_audit":          {"old_value": true, "new_value": true},
	"config.traeger_setting_audit":  {"old_value": true, "new_value": true},
}

var errSettingExportField = errors.New("settings: not a projected setting export field")

// SettingExportFields projects setting values for the return export of a
// school or Träger (#3838, Anhang C AGB "Kundenspezifische Einstellungen").
// A value of a registered, non-secret definition leaves as stored. A secret
// (FieldPassword: device PIN, SFTP password) and a key the registry no longer
// knows leave as null: their meaning, and so whether they are secret, is not
// known to the export.
type SettingExportFields struct{}

func (SettingExportFields) ProjectExportField(table, column string, row map[string]json.RawMessage, raw json.RawMessage) (json.RawMessage, error) {
	if !settingExportColumns[table][column] {
		return nil, errSettingExportField
	}
	var key string
	if json.Unmarshal(row["setting_key"], &key) != nil || key == "" {
		return nil, errSettingExportField
	}
	definition := configModel.GetDefinition(key)
	if definition == nil || definition.Type == configModel.FieldPassword {
		return json.RawMessage("null"), nil
	}
	return raw, nil
}
