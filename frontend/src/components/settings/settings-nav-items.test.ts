import { describe, expect, it } from "vitest";
import { settingsConcept } from "~/lib/concepts/settings";
import {
  activeSettingsSectionId,
  settingsSectionHref,
} from "./settings-nav-items";

const tabs = [
  { id: "settings-operations", label: "Betrieb", icon: settingsConcept },
  { id: "settings-gdpr", label: "Datenschutz", icon: settingsConcept },
  { id: "settings-devices", label: "Geräte", icon: settingsConcept },
];

describe("activeSettingsSectionId", () => {
  it("prefers the section the address names", () => {
    expect(
      activeSettingsSectionId(
        { tabs, highlightTabId: "settings-devices" },
        "gdpr",
      ),
    ).toBe("settings-gdpr");
  });

  it("uses the section of a deep-linked setting without ?tab=", () => {
    expect(
      activeSettingsSectionId(
        { tabs, highlightTabId: "settings-devices" },
        null,
      ),
    ).toBe("settings-devices");
  });

  it("falls back to the first section for an unknown or hidden value", () => {
    expect(activeSettingsSectionId({ tabs }, "gibtsnicht")).toBe(
      "settings-operations",
    );
    expect(
      activeSettingsSectionId({ tabs, highlightTabId: "settings-x" }, null),
    ).toBe("settings-operations");
  });

  it("has no section when the list is empty", () => {
    expect(activeSettingsSectionId({ tabs: [] }, null)).toBe("");
  });
});

describe("settingsSectionHref", () => {
  it("names the section in the address", () => {
    expect(settingsSectionHref("settings-gdpr")).toBe("/settings?tab=gdpr");
  });
});
