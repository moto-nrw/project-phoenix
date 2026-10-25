import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ResolvedSetting } from "~/lib/settings-api";
import { SettingsField } from "./settings-field";

function moduleSetting(
  overrides: Partial<ResolvedSetting> = {},
): ResolvedSetting {
  return {
    key: "module.zeiterfassung",
    label: "Zeiterfassung",
    description:
      "Stempeluhr, Arbeitszeitmodelle, Urlaub und Export für das Lohnbüro.",
    type: "boolean",
    default: true,
    value: true,
    is_default: true,
    writable: false,
    visible: true,
    sort_order: 6,
    access_policy: "operator_managed",
    levels: "schule",
    source: "default",
    validation: null,
    depends_on: null,
    options: null,
    ...overrides,
  };
}

function renderField(
  setting: ResolvedSetting,
  audience: "admin" | "operator" = "admin",
) {
  return render(
    <ToastProvider>
      <SettingsField
        setting={setting}
        audience={audience}
        onSave={vi.fn()}
        onReset={vi.fn()}
      />
    </ToastProvider>,
  );
}

// A module switch (#4892) names the booking, not a retention window.
describe("SettingsField for a module switch", () => {
  it("tells the school that the moto team changes its modules", () => {
    const { getByText, queryByText, getByRole } = renderField(moduleSetting());

    expect(getByText("Nur Lesen")).toBeInTheDocument();
    expect(
      getByText(
        "Welche Module Ihre Schule hat, steht im Vertrag mit moto. Für eine Änderung wenden Sie sich an das moto-Team.",
      ),
    ).toBeInTheDocument();
    expect(queryByText(/Frist/)).not.toBeInTheDocument();
    expect(getByRole("switch", { name: "Zeiterfassung" })).toBeDisabled();
  });

  it("tells the moto team that the school only sees the booking", () => {
    const { getByText, queryByText } = renderField(
      moduleSetting({ writable: true }),
      "operator",
    );

    expect(
      getByText(
        "Gebuchtes Modul aus dem Vertrag. Die Schule sieht es, kann es aber nicht ändern.",
      ),
    ).toBeInTheDocument();
    expect(queryByText(/Löschfrist/)).not.toBeInTheDocument();
  });
});
