import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ResolvedSetting } from "~/lib/settings-api";
import { SettingsField } from "./settings-field";

// Werte des Trägers zwischen Standard und Schule (#4652, ADR 0086).

function makeSetting(
  overrides: Partial<ResolvedSetting> = {},
): ResolvedSetting {
  return {
    key: "parent.news_enabled",
    label: "Neuigkeiten im Elternportal",
    description: "Eltern sehen die Neuigkeiten der OGS.",
    type: "boolean",
    default: false,
    value: true,
    is_default: true,
    writable: true,
    visible: true,
    sort_order: 1,
    access_policy: "shared",
    levels: "beide",
    source: "traeger",
    traeger_changed_at: "2026-09-01T08:00:00Z",
    validation: null,
    depends_on: null,
    options: null,
    ...overrides,
  };
}

function renderField(
  setting: ResolvedSetting,
  audience?: "admin" | "operator" | "traeger",
  onReset = vi.fn().mockResolvedValue(undefined),
) {
  render(
    <ToastProvider>
      <SettingsField
        setting={setting}
        audience={audience}
        onSave={vi.fn().mockResolvedValue(undefined)}
        onReset={onReset}
      />
    </ToastProvider>,
  );
  return onReset;
}

describe("SettingsField at the school, with a Träger value", () => {
  it("says the Träger decided the value and when", () => {
    renderField(makeSetting());

    expect(
      screen.getByText("Vom Träger festgelegt · geändert am 01.09.2026"),
    ).toBeInTheDocument();
    // Anker: die Einstellung ist gerendert; „Standard“ wäre hier falsch.
    expect(screen.getByText("Neuigkeiten im Elternportal")).toBeInTheDocument();
    expect(screen.queryByText("Standard")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Zurücksetzen/ }),
    ).not.toBeInTheDocument();
  });

  it("shows Standard where neither school nor Träger set a value", () => {
    renderField(
      makeSetting({
        value: false,
        source: "default",
        traeger_changed_at: undefined,
      }),
    );

    expect(screen.getByText("Standard")).toBeInTheDocument();
    expect(screen.queryByText(/Vom Träger festgelegt/)).not.toBeInTheDocument();
  });

  it("marks the school's own value and resets it to the Träger value", async () => {
    const onReset = renderField(
      makeSetting({ value: false, is_default: false, source: "school" }),
    );

    expect(
      screen.getByText(
        "Eigener Wert Ihrer Schule. „Zurücksetzen“ übernimmt den Wert des Trägers.",
      ),
    ).toBeInTheDocument();
    // A toggle normally has no reset; back to the Träger value it does.
    const reset = screen.getByTitle("Auf den Wert des Trägers zurücksetzen");
    fireEvent.click(reset);

    await waitFor(() =>
      expect(onReset).toHaveBeenCalledWith("parent.news_enabled"),
    );
    expect(
      await screen.findByText(
        "Für „Neuigkeiten im Elternportal“ gilt wieder der Wert des Trägers.",
      ),
    ).toBeInTheDocument();
  });

  it("names the school for the moto team", () => {
    renderField(
      makeSetting({ value: false, is_default: false, source: "school" }),
      "operator",
    );

    expect(
      screen.getByText(
        "Eigener Wert der Schule. „Zurücksetzen“ übernimmt den Wert des Trägers.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a Träger-only setting read-only with who decides it", () => {
    renderField(
      makeSetting({ levels: "traeger", writable: false, value: false }),
    );

    expect(
      screen.getByText("Legt der Träger fest · geändert am 01.09.2026"),
    ).toBeInTheDocument();
    expect(screen.getByText("Nur Lesen")).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Neuigkeiten im Elternportal" }),
    ).toBeDisabled();
  });
});

describe("SettingsField in the Träger portal", () => {
  it("offers to remove the Träger value of a toggle", async () => {
    const onReset = renderField(makeSetting({ is_default: false }), "traeger");

    expect(
      screen.getByText("Vom Träger festgelegt · geändert am 01.09.2026"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByTitle("Auf Standard zurücksetzen"));

    await waitFor(() =>
      expect(onReset).toHaveBeenCalledWith("parent.news_enabled"),
    );
    expect(
      await screen.findByText(
        "Die Einstellung „Neuigkeiten im Elternportal“ steht wieder auf dem Standard.",
      ),
    ).toBeInTheDocument();
  });

  it("shows Standard without a Träger value and offers no reset", () => {
    renderField(
      makeSetting({
        value: false,
        source: "default",
        traeger_changed_at: undefined,
      }),
      "traeger",
    );

    expect(screen.getByText("Standard")).toBeInTheDocument();
    expect(screen.queryByTitle("Auf Standard zurücksetzen")).toBeNull();
  });

  it("edits the AGB as text only; the PDF stays with each school", () => {
    renderField(
      makeSetting({
        key: "enrollment.legal_agb_text",
        label: "AGB / Teilnahmebedingungen",
        type: "textarea",
        default: "",
        value: "Unsere AGB",
        is_default: false,
      }),
      "traeger",
    );

    fireEvent.click(screen.getByRole("button", { name: /AGB überarbeiten/ }));

    expect(screen.getByLabelText("AGB-Text")).toHaveValue("Unsere AGB");
    expect(screen.queryByText("PDF-Datei hochladen")).not.toBeInTheDocument();
  });
});
