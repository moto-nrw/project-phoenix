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
  it.each([false, true])(
    "preserves dirty text while a refreshed schema updates pristine fields (dirty=%s)",
    (dirty) => {
      const setting = makeSetting({
        key: "branding.letterhead_text",
        label: "Absender",
        type: "textarea",
        value: "Gespeichert",
        default: "",
      });
      const onSave = vi.fn().mockResolvedValue(undefined);
      const field = (value: string) => (
        <ToastProvider>
          <SettingsField
            setting={{ ...setting, value }}
            audience="traeger"
            onSave={onSave}
            onReset={vi.fn()}
          />
        </ToastProvider>
      );
      const view = render(field("Gespeichert"));
      if (dirty)
        fireEvent.change(screen.getByLabelText("Absender"), {
          target: { value: "Mein Entwurf" },
        });
      view.rerender(field("Neuer Serverwert"));
      expect(screen.getByLabelText("Absender")).toHaveValue(
        dirty ? "Mein Entwurf" : "Neuer Serverwert",
      );
    },
  );

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

  it("offers a reset of the school's own toggle while the Träger has no value", async () => {
    const onReset = renderField(
      makeSetting({
        value: false,
        is_default: true,
        source: "school",
        traeger_changed_at: undefined,
      }),
    );

    // Back to Standard, so a later Träger value reaches the school again.
    const reset = screen.getByTitle("Auf Standard zurücksetzen");
    expect(screen.queryByText("Standard")).not.toBeInTheDocument();
    fireEvent.click(reset);
    await waitFor(() =>
      expect(onReset).toHaveBeenCalledWith("parent.news_enabled"),
    );
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

describe("SettingsField with a colour", () => {
  it("shows the colour picker and the code field with the same value", () => {
    renderField(
      makeSetting({
        key: "branding.primary_color",
        label: "Farbe für Briefe und Rechnungen",
        type: "color",
        default: "#111827",
        value: "#217A78",
        levels: "traeger",
      }),
      "traeger",
    );

    const picker = screen.getByLabelText("Farbe für Briefe und Rechnungen");
    expect(picker).toHaveAttribute("type", "color");
    expect(picker).toHaveValue("#217a78");
    expect(
      screen.getByLabelText("Farbe für Briefe und Rechnungen als Farbcode"),
    ).toHaveValue("#217A78");
  });

  it("puts the picked colour into the code field", () => {
    renderField(
      makeSetting({
        key: "branding.primary_color",
        label: "Farbe",
        type: "color",
        default: "#111827",
        value: "#111827",
        levels: "traeger",
      }),
      "traeger",
    );

    fireEvent.change(screen.getByLabelText("Farbe"), {
      target: { value: "#1f4e79" },
    });
    expect(screen.getByLabelText("Farbe als Farbcode")).toHaveValue("#1F4E79");
  });

  it("keeps the default in the picker while the typed code is incomplete", () => {
    renderField(
      makeSetting({
        key: "branding.primary_color",
        label: "Farbe",
        type: "color",
        default: "#111827",
        value: "#12",
        levels: "traeger",
      }),
      "traeger",
    );

    expect(screen.getByLabelText("Farbe")).toHaveValue("#111827");
    expect(screen.getByLabelText("Farbe als Farbcode")).toHaveValue("#12");
  });
});

describe("SettingsField with colour presets", () => {
  it("offers the presets of the setting and saves the picked one", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <ToastProvider>
        <SettingsField
          setting={makeSetting({
            key: "branding.primary_color",
            label: "Farbe für Briefe und Rechnungen",
            type: "color",
            default: "#111827",
            value: "#111827",
            levels: "traeger",
            options: {
              static: [
                { label: "Schwarz", value: "#111827" },
                { label: "Dunkelblau", value: "#1F4E79" },
              ],
            },
          })}
          audience="traeger"
          onSave={onSave}
          onReset={vi.fn().mockResolvedValue(undefined)}
        />
      </ToastProvider>,
    );

    expect(screen.getByRole("radio", { name: "Schwarz" })).toBeChecked();
    expect(screen.queryByLabelText(/als Farbcode/)).not.toBeInTheDocument();

    const darkBlue = screen.getByRole("radio", { name: "Dunkelblau" });
    fireEvent.click(darkBlue);
    fireEvent.blur(darkBlue);

    await waitFor(() =>
      expect(onSave).toHaveBeenCalledWith("branding.primary_color", "#1F4E79"),
    );
  });
});
