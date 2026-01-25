import { describe, it, expect } from "vitest";
import {
  mapDefinitionResponse,
  mapResolvedSettingResponse,
  mapSettingChangeResponse,
  groupSettingsByCategory,
  getCategoryLabel,
  getScopeLabel,
  getSourceLabel,
  valueToString,
  isSettingActive,
  type BackendSettingDefinition,
  type BackendResolvedSetting,
  type BackendSettingChange,
  type ResolvedSetting,
} from "./settings-helpers";

// === Sample Backend Data ===

const sampleBackendDefinition: BackendSettingDefinition = {
  id: 1,
  key: "session.auto_checkout_enabled",
  type: "bool",
  default_value: true,
  category: "session",
  description: "Automatisches Auschecken aktivieren",
  validation: { min: 0, max: 100 },
  allowed_scopes: ["system", "og", "user"],
  scope_permissions: { system: "admin", og: "og_admin" },
  depends_on: { key: "session.enabled", condition: "equals", value: true },
  group_name: "Checkout",
  sort_order: 10,
};

const sampleBackendResolvedSetting: BackendResolvedSetting = {
  key: "session.auto_checkout_enabled",
  value: true,
  type: "bool",
  category: "session",
  description: "Automatisches Auschecken aktivieren",
  group_name: "Checkout",
  source: { type: "system", id: 1 },
  is_default: false,
  is_active: true,
  can_modify: true,
  depends_on: { key: "session.enabled", condition: "equals", value: true },
  validation: { options: ["option1", "option2"] },
};

const sampleBackendSettingChange: BackendSettingChange = {
  id: 100,
  setting_key: "session.auto_checkout_enabled",
  scope_type: "system",
  scope_id: 1,
  change_type: "update",
  old_value: false,
  new_value: true,
  reason: "Enabling auto checkout",
  account_id: 5,
  account_email: "admin@example.com",
  created_at: "2024-01-15T10:30:00Z",
};

// === Mapping Tests ===

describe("mapDefinitionResponse", () => {
  it("maps backend definition to frontend structure", () => {
    const result = mapDefinitionResponse(sampleBackendDefinition);

    expect(result.id).toBe("1"); // int64 → string
    expect(result.key).toBe("session.auto_checkout_enabled");
    expect(result.type).toBe("bool");
    expect(result.defaultValue).toBe(true);
    expect(result.category).toBe("session");
    expect(result.description).toBe("Automatisches Auschecken aktivieren");
    expect(result.allowedScopes).toEqual(["system", "og", "user"]);
    expect(result.scopePermissions).toEqual({
      system: "admin",
      og: "og_admin",
    });
    expect(result.groupName).toBe("Checkout");
    expect(result.sortOrder).toBe(10);
  });

  it("maps validation correctly", () => {
    const result = mapDefinitionResponse(sampleBackendDefinition);

    expect(result.validation?.min).toBe(0);
    expect(result.validation?.max).toBe(100);
  });

  it("maps dependency correctly", () => {
    const result = mapDefinitionResponse(sampleBackendDefinition);

    expect(result.dependsOn?.key).toBe("session.enabled");
    expect(result.dependsOn?.condition).toBe("equals");
    expect(result.dependsOn?.value).toBe(true);
  });

  it("handles definition without optional fields", () => {
    const minimalDefinition: BackendSettingDefinition = {
      id: 2,
      key: "minimal.setting",
      type: "string",
      default_value: "",
      category: "general",
      allowed_scopes: ["system"],
      scope_permissions: {},
      sort_order: 0,
    };

    const result = mapDefinitionResponse(minimalDefinition);

    expect(result.description).toBeUndefined();
    expect(result.validation).toBeUndefined();
    expect(result.dependsOn).toBeUndefined();
    expect(result.groupName).toBeUndefined();
  });
});

describe("mapResolvedSettingResponse", () => {
  it("maps backend resolved setting to frontend structure", () => {
    const result = mapResolvedSettingResponse(sampleBackendResolvedSetting);

    expect(result.key).toBe("session.auto_checkout_enabled");
    expect(result.value).toBe(true);
    expect(result.type).toBe("bool");
    expect(result.category).toBe("session");
    expect(result.description).toBe("Automatisches Auschecken aktivieren");
    expect(result.groupName).toBe("Checkout");
    expect(result.isDefault).toBe(false);
    expect(result.isActive).toBe(true);
    expect(result.canModify).toBe(true);
  });

  it("maps source correctly", () => {
    const result = mapResolvedSettingResponse(sampleBackendResolvedSetting);

    expect(result.source?.type).toBe("system");
    expect(result.source?.id).toBe("1"); // int → string
  });

  it("maps validation with options", () => {
    const result = mapResolvedSettingResponse(sampleBackendResolvedSetting);

    expect(result.validation?.options).toEqual(["option1", "option2"]);
  });

  it("handles setting without source", () => {
    const settingWithoutSource: BackendResolvedSetting = {
      ...sampleBackendResolvedSetting,
      source: undefined,
    };

    const result = mapResolvedSettingResponse(settingWithoutSource);

    expect(result.source).toBeUndefined();
  });
});

describe("mapSettingChangeResponse", () => {
  it("maps backend setting change to frontend structure", () => {
    const result = mapSettingChangeResponse(sampleBackendSettingChange);

    expect(result.id).toBe("100");
    expect(result.settingKey).toBe("session.auto_checkout_enabled");
    expect(result.scopeType).toBe("system");
    expect(result.scopeId).toBe("1");
    expect(result.changeType).toBe("update");
    expect(result.oldValue).toBe(false);
    expect(result.newValue).toBe(true);
    expect(result.reason).toBe("Enabling auto checkout");
    expect(result.accountId).toBe("5");
    expect(result.accountEmail).toBe("admin@example.com");
    expect(result.createdAt).toBeInstanceOf(Date);
  });

  it("handles change without optional fields", () => {
    const minimalChange: BackendSettingChange = {
      id: 101,
      setting_key: "test.key",
      scope_type: "user",
      change_type: "create",
      created_at: "2024-01-01T00:00:00Z",
    };

    const result = mapSettingChangeResponse(minimalChange);

    expect(result.scopeId).toBeUndefined();
    expect(result.oldValue).toBeUndefined();
    expect(result.newValue).toBeUndefined();
    expect(result.reason).toBeUndefined();
    expect(result.accountId).toBeUndefined();
    expect(result.accountEmail).toBeUndefined();
  });
});

// === Grouping Tests ===

describe("groupSettingsByCategory", () => {
  const settings: ResolvedSetting[] = [
    {
      key: "session.timeout",
      value: 30,
      type: "int",
      category: "session",
      groupName: "Timeouts",
      isDefault: true,
      isActive: true,
      canModify: true,
    },
    {
      key: "session.auto_checkout",
      value: true,
      type: "bool",
      category: "session",
      groupName: "Checkout",
      isDefault: false,
      isActive: true,
      canModify: true,
    },
    {
      key: "appearance.theme",
      value: "light",
      type: "enum",
      category: "appearance",
      isDefault: true,
      isActive: true,
      canModify: true,
    },
  ];

  it("groups settings by category", () => {
    const result = groupSettingsByCategory(settings);

    expect(result).toHaveLength(2);
    expect(result.map((c) => c.name)).toContain("session");
    expect(result.map((c) => c.name)).toContain("appearance");
  });

  it("groups settings within category by group_name", () => {
    const result = groupSettingsByCategory(settings);
    const sessionCategory = result.find((c) => c.name === "session");

    expect(sessionCategory?.groups).toHaveLength(2);
    expect(sessionCategory?.groups.map((g) => g.name)).toContain("Timeouts");
    expect(sessionCategory?.groups.map((g) => g.name)).toContain("Checkout");
  });

  it("uses _ungrouped for settings without group_name", () => {
    const result = groupSettingsByCategory(settings);
    const appearanceCategory = result.find((c) => c.name === "appearance");

    expect(appearanceCategory?.groups).toHaveLength(1);
    expect(appearanceCategory?.groups[0]?.name).toBe("_ungrouped");
  });

  it("handles empty settings array", () => {
    const result = groupSettingsByCategory([]);

    expect(result).toEqual([]);
  });
});

// === Display Helper Tests ===

describe("getCategoryLabel", () => {
  it("returns German label for known categories", () => {
    expect(getCategoryLabel("session")).toBe("Sitzung");
    expect(getCategoryLabel("pickup")).toBe("Abholung");
    expect(getCategoryLabel("notifications")).toBe("Benachrichtigungen");
    expect(getCategoryLabel("appearance")).toBe("Darstellung");
    expect(getCategoryLabel("audit")).toBe("Protokollierung");
    expect(getCategoryLabel("checkin")).toBe("Check-in");
    expect(getCategoryLabel("device")).toBe("Gerät");
  });

  it("returns original value for unknown categories", () => {
    expect(getCategoryLabel("unknown")).toBe("unknown");
    expect(getCategoryLabel("custom_category")).toBe("custom_category");
  });
});

describe("getScopeLabel", () => {
  it("returns German label for known scopes", () => {
    expect(getScopeLabel("system")).toBe("System");
    expect(getScopeLabel("school")).toBe("Schule");
    expect(getScopeLabel("og")).toBe("OG");
    expect(getScopeLabel("user")).toBe("Benutzer");
    expect(getScopeLabel("device")).toBe("Gerät");
  });

  it("returns original value for unknown scopes", () => {
    expect(getScopeLabel("unknown")).toBe("unknown");
  });
});

describe("getSourceLabel", () => {
  it("returns 'Standard' for default settings", () => {
    const setting: ResolvedSetting = {
      key: "test",
      value: true,
      type: "bool",
      category: "test",
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(getSourceLabel(setting)).toBe("Standard");
  });

  it("returns inherited label when source is present", () => {
    const setting: ResolvedSetting = {
      key: "test",
      value: true,
      type: "bool",
      category: "test",
      source: { type: "system" },
      isDefault: false,
      isActive: true,
      canModify: true,
    };

    expect(getSourceLabel(setting)).toBe("Von System geerbt");
  });

  it("returns 'Eigener Wert' for custom value without source", () => {
    const setting: ResolvedSetting = {
      key: "test",
      value: true,
      type: "bool",
      category: "test",
      isDefault: false,
      isActive: true,
      canModify: true,
    };

    expect(getSourceLabel(setting)).toBe("Eigener Wert");
  });
});

// === Utility Function Tests ===

describe("valueToString", () => {
  it("returns empty string for null", () => {
    expect(valueToString(null)).toBe("");
  });

  it("returns empty string for undefined", () => {
    expect(valueToString(undefined)).toBe("");
  });

  it("converts string to string", () => {
    expect(valueToString("hello")).toBe("hello");
  });

  it("converts number to string", () => {
    expect(valueToString(42)).toBe("42");
    expect(valueToString(3.14)).toBe("3.14");
    expect(valueToString(0)).toBe("0");
  });

  it("returns empty string for objects", () => {
    expect(valueToString({ key: "value" })).toBe("");
  });

  it("returns empty string for arrays", () => {
    expect(valueToString([1, 2, 3])).toBe("");
  });

  it("returns empty string for booleans", () => {
    expect(valueToString(true)).toBe("");
    expect(valueToString(false)).toBe("");
  });
});

describe("isSettingActive", () => {
  const allSettings: ResolvedSetting[] = [
    {
      key: "parent.enabled",
      value: true,
      type: "bool",
      category: "test",
      isDefault: true,
      isActive: true,
      canModify: true,
    },
    {
      key: "parent.mode",
      value: "advanced",
      type: "enum",
      category: "test",
      isDefault: true,
      isActive: true,
      canModify: true,
    },
    {
      key: "parent.value",
      value: "something",
      type: "string",
      category: "test",
      isDefault: true,
      isActive: true,
      canModify: true,
    },
  ];

  it("returns true for settings without dependencies", () => {
    const setting: ResolvedSetting = {
      key: "independent",
      value: true,
      type: "bool",
      category: "test",
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(true);
  });

  it("handles 'equals' condition - true case", () => {
    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: { key: "parent.enabled", condition: "equals", value: true },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(true);
  });

  it("handles 'equals' condition - false case", () => {
    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: { key: "parent.enabled", condition: "equals", value: false },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(false);
  });

  it("handles 'not_equals' condition", () => {
    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: {
        key: "parent.mode",
        condition: "not_equals",
        value: "basic",
      },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(true);
  });

  it("handles 'in' condition - value in array", () => {
    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: {
        key: "parent.mode",
        condition: "in",
        value: ["basic", "advanced", "expert"],
      },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(true);
  });

  it("handles 'in' condition - value not in array", () => {
    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: {
        key: "parent.mode",
        condition: "in",
        value: ["basic", "simple"],
      },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(false);
  });

  it("handles 'not_empty' condition - non-empty value", () => {
    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: { key: "parent.value", condition: "not_empty", value: null },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(true);
  });

  it("handles 'not_empty' condition - empty string", () => {
    const settingsWithEmpty: ResolvedSetting[] = [
      {
        key: "parent.value",
        value: "",
        type: "string",
        category: "test",
        isDefault: true,
        isActive: true,
        canModify: true,
      },
    ];

    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: { key: "parent.value", condition: "not_empty", value: null },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, settingsWithEmpty)).toBe(false);
  });

  it("returns true when dependent setting is not found", () => {
    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: { key: "nonexistent", condition: "equals", value: true },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(true);
  });

  it("returns true for unknown condition", () => {
    const setting: ResolvedSetting = {
      key: "dependent",
      value: "test",
      type: "string",
      category: "test",
      dependsOn: {
        key: "parent.enabled",
        condition: "unknown_condition",
        value: true,
      },
      isDefault: true,
      isActive: true,
      canModify: true,
    };

    expect(isSettingActive(setting, allSettings)).toBe(true);
  });
});
