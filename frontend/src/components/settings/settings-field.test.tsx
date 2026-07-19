import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, waitFor } from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";
import { SettingsField } from "./settings-field";
import type { ResolvedSetting } from "~/lib/settings-api";

function renderWithProviders(ui: React.ReactElement) {
  return render(<ToastProvider>{ui}</ToastProvider>);
}

function makeSetting(
  overrides: Partial<ResolvedSetting> = {},
): ResolvedSetting {
  return {
    key: "test.setting",
    label: "Test Setting",
    description: "A test setting",
    type: "text",
    default: "default",
    value: "current",
    is_default: false,
    writable: true,
    visible: true,
    sort_order: 1,
    access_policy: "shared",
    validation: null,
    depends_on: null,
    options: null,
    ...overrides,
  };
}

describe("SettingsField", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders label and description", () => {
    const { getByText } = renderWithProviders(
      <SettingsField
        setting={makeSetting()}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Test Setting")).toBeDefined();
    expect(getByText("A test setting")).toBeDefined();
  });

  it("shows Standard badge when is_default", () => {
    const { getByText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ is_default: true })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Standard")).toBeDefined();
  });

  it("shows Nur Lesen badge when not writable", () => {
    const { getByText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ writable: false })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Nur Lesen")).toBeDefined();
  });

  it("renders nothing when not visible", () => {
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ visible: false })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(container.querySelector("label")).toBeNull();
    expect(container.querySelector("input")).toBeNull();
  });

  it("shows reset button when not default", () => {
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ is_default: false })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    const resetBtn = container.querySelector("button[title]");
    expect(resetBtn).toBeDefined();
  });

  it("hides reset button when is_default", () => {
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ is_default: true })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    const resetBtn = container.querySelector(
      "button[title='Auf Standard zurücksetzen']",
    );
    expect(resetBtn).toBeNull();
  });

  it("hides reset button for enrollment legal text fields", () => {
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "enrollment.legal_dsgvo_text",
          type: "textarea",
          value: "Datenschutz Text",
          is_default: false,
        })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    const resetBtn = container.querySelector(
      "button[title='Auf Standard zurücksetzen']",
    );
    expect(resetBtn).toBeNull();
  });

  it("renders boolean field as toggle", () => {
    const { getByRole } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ type: "boolean", value: true })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByRole("switch")).toBeDefined();
  });

  it("renders number field", () => {
    const { getByRole } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ type: "number", value: 42 })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect((getByRole("spinbutton") as HTMLInputElement).value).toBe("42");
  });

  it("renders time field", () => {
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ type: "time", value: "18:00" })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    const input = container.querySelector(
      "input[placeholder='HH:MM']",
    ) as HTMLInputElement;
    expect(input).not.toBeNull();
    expect(input.value).toBe("18:00");
  });

  it("renders password field with masked display", () => {
    const { getByText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ type: "password", value: "secret" })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("••••••")).toBeDefined();
  });

  it("renders select field", () => {
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          type: "select",
          value: "de",
          options: {
            static: [
              { label: "Deutsch", value: "de" },
              { label: "English", value: "en" },
            ],
          },
        })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(container.querySelector("select")).toBeDefined();
  });

  it("saves boolean immediately on toggle", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const { getByRole } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ type: "boolean", value: false })}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    fireEvent.click(getByRole("switch"));
    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith("test.setting", true);
    });
  });

  it("saves text on blur (not on keystroke)", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ type: "text", value: "old" })}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    const input = container.querySelector(
      "input[type='text']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "new" } });

    // Should NOT have saved yet (no blur, no debounce timeout)
    expect(onSave).not.toHaveBeenCalled();

    // Blur triggers save
    fireEvent.blur(input);
    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith("test.setting", "new");
    });
  });

  it("does not save on keystroke (only on blur or debounce)", () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ type: "number", value: 10 })}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    const input = container.querySelector(
      "input[type='number']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "20" } });

    // Should NOT have saved immediately
    expect(onSave).not.toHaveBeenCalled();
  });

  it("saves a dirty field with its own validation context when the setting changes", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const onReset = vi.fn().mockResolvedValue(null);
    const { container, rerender } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ key: "text.setting", value: "old" })}
        onSave={onSave}
        onReset={onReset}
      />,
    );

    const input = container.querySelector(
      "input[type='text']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "unsaved text" } });

    rerender(
      <ToastProvider>
        <SettingsField
          setting={makeSetting({
            key: "number.setting",
            type: "number",
            value: 10,
          })}
          onSave={onSave}
          onReset={onReset}
        />
      </ToastProvider>,
    );

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith("text.setting", "unsaved text");
    });
  });

  it("calls onReset when reset button clicked", async () => {
    const onReset = vi.fn().mockResolvedValue(null);
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ is_default: false })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={onReset}
      />,
    );

    const resetBtn = container.querySelector(
      "button[title='Auf Standard zurücksetzen']",
    );
    expect(resetBtn).not.toBeNull();
    fireEvent.click(resetBtn!);

    await waitFor(() => {
      expect(onReset).toHaveBeenCalledWith("test.setting");
    });
  });

  it("shows validation error for invalid number", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          type: "number",
          value: 50,
          validation: { min: 10, max: 100 },
        })}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    const input = container.querySelector(
      "input[type='number']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "5" } });
    fireEvent.blur(input);

    await waitFor(() => {
      const errorText = container.querySelector(".text-red-600");
      expect(errorText).not.toBeNull();
      expect(errorText!.textContent).toBe("Minimum: 10");
    });
    expect(onSave).not.toHaveBeenCalled();
  });

  it("shows error message from failed save", async () => {
    const onSave = vi.fn().mockResolvedValue("Ungültiger Wert.");
    const { getByRole, container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({ type: "boolean", value: false })}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    fireEvent.click(getByRole("switch"));
    await waitFor(() => {
      const errorText = container.querySelector(".text-red-600");
      expect(errorText).not.toBeNull();
      expect(errorText!.textContent).toBe("Ungültiger Wert.");
    });
  });

  it("warns when an enabled enrollment legal block has no text", () => {
    const { getByText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "enrollment.legal_dsgvo_enabled",
          label: "Datenschutzinformation anzeigen",
          type: "boolean",
          value: true,
        })}
        categoryItems={[
          makeSetting({
            key: "enrollment.legal_dsgvo_text",
            type: "textarea",
            value: "",
          }),
        ]}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    expect(
      getByText(
        "Wird erst im Anmeldeformular angezeigt, wenn der passende Text hinterlegt ist.",
      ),
    ).toBeDefined();
  });

  it("edits enrollment legal text in a modal and blocks empty saves", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const { container, getByRole, getByLabelText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "enrollment.legal_dsgvo_text",
          label: "Datenschutzinformation Text",
          type: "textarea",
          value: "Datenschutz Text",
        })}
        categoryItems={[
          makeSetting({
            key: "enrollment.legal_dsgvo_enabled",
            type: "boolean",
            value: true,
          }),
        ]}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    expect(container.querySelector("textarea")).toBeNull();

    fireEvent.click(getByRole("button", { name: "Rechtstext bearbeiten" }));

    const textarea = getByLabelText("Rechtstext");
    expect((textarea as HTMLTextAreaElement).value).toBe("Datenschutz Text");

    fireEvent.change(textarea, { target: { value: "" } });
    expect(getByRole("button", { name: "Speichern" })).toBeDisabled();
    expect(document.body.textContent).toContain(
      "Dieser Text oder eine PDF-Datei ist erforderlich, solange der Block im Anmeldeformular angezeigt wird.",
    );
    expect(onSave).not.toHaveBeenCalled();

    fireEvent.change(textarea, { target: { value: "Neuer Datenschutz Text" } });
    fireEvent.click(getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(
        "enrollment.legal_dsgvo_text",
        "Neuer Datenschutz Text",
      );
    });
  });

  it("opens a required text modal before enabling an enrollment legal block", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const { getByRole, getByLabelText, queryByRole } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "enrollment.legal_dsgvo_enabled",
          label: "Datenschutzinformation anzeigen",
          type: "boolean",
          value: false,
        })}
        categoryItems={[
          makeSetting({
            key: "enrollment.legal_dsgvo_text",
            type: "textarea",
            value: "",
          }),
        ]}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    fireEvent.click(getByRole("switch"));

    expect(
      getByRole("heading", {
        name: "Datenschutzinformation aktivieren",
      }),
    ).toBeDefined();
    expect(getByLabelText("Rechtstext")).toBeDefined();
    expect(getByRole("button", { name: "Aktivieren" })).toBeDisabled();
    expect(onSave).not.toHaveBeenCalled();

    fireEvent.change(getByLabelText("Rechtstext"), {
      target: { value: "Datenschutz Text" },
    });
    fireEvent.click(getByRole("button", { name: "Aktivieren" }));

    await waitFor(() => {
      expect(onSave).toHaveBeenNthCalledWith(
        1,
        "enrollment.legal_dsgvo_text",
        "Datenschutz Text",
      );
      expect(onSave).toHaveBeenNthCalledWith(
        2,
        "enrollment.legal_dsgvo_enabled",
        true,
      );
    });
    await waitFor(() => {
      expect(
        queryByRole("heading", {
          name: "Datenschutzinformation aktivieren",
        }),
      ).toBeNull();
    });
  });

  it("shows the current AGB source and keeps the PDF open action inside the edit modal", () => {
    const { getByRole, getByText, queryByRole } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "enrollment.legal_agb_text",
          label: "AGB / Teilnahmebedingungen",
          type: "textarea",
          value: "Gespeicherter AGB-Text",
        })}
        categoryItems={[
          makeSetting({
            key: "enrollment.legal_agb_document_url",
            type: "text",
            value: "/uploads/enrollment-legal-documents/terms.pdf",
          }),
          makeSetting({
            key: "enrollment.legal_agb_display_mode",
            type: "select",
            value: "pdf",
          }),
        ]}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    expect(
      getByText((_, element) => element?.textContent === "Quelle: PDF-Datei"),
    ).toBeDefined();
    expect(queryByRole("link", { name: "Öffnen" })).toBeNull();

    fireEvent.click(getByRole("button", { name: "AGB überarbeiten" }));

    expect(getByRole("link", { name: "Öffnen" })).toHaveAttribute(
      "href",
      "/api/public/enrollment-legal-documents/terms.pdf",
    );
    expect(getByText("PDF ersetzen")).toBeDefined();
    expect(getByRole("button", { name: "Entfernen" })).toBeDefined();
  });

  it("hides AGB PDF upload and delete controls for operator settings", () => {
    const { getByRole, getByText, queryByText, queryByRole } =
      renderWithProviders(
        <SettingsField
          setting={makeSetting({
            key: "enrollment.legal_agb_text",
            label: "AGB / Teilnahmebedingungen",
            type: "textarea",
            value: "Gespeicherter AGB-Text",
          })}
          categoryItems={[
            makeSetting({
              key: "enrollment.legal_agb_document_url",
              type: "text",
              value: "/uploads/enrollment-legal-documents/terms.pdf",
            }),
            makeSetting({
              key: "enrollment.legal_agb_display_mode",
              type: "select",
              value: "pdf",
            }),
          ]}
          onSave={vi.fn().mockResolvedValue(null)}
          onReset={vi.fn().mockResolvedValue(null)}
          audience="operator"
        />,
      );

    fireEvent.click(getByRole("button", { name: "AGB überarbeiten" }));

    expect(getByRole("link", { name: "Öffnen" })).toHaveAttribute(
      "href",
      "/api/public/enrollment-legal-documents/terms.pdf",
    );
    expect(queryByText("PDF ersetzen")).toBeNull();
    expect(queryByRole("button", { name: "Entfernen" })).toBeNull();
    expect(
      getByText(
        "PDF-Dateien können nur im Schulportal hochgeladen oder entfernt werden.",
      ),
    ).toBeDefined();
  });

  it("hides only the delete control when active AGB terms use PDF mode", () => {
    const { getByRole, getByText, queryByRole } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "enrollment.legal_agb_text",
          label: "AGB / Teilnahmebedingungen",
          type: "textarea",
          value: "Gespeicherter AGB-Text",
        })}
        categoryItems={[
          makeSetting({
            key: "enrollment.legal_terms_enabled",
            type: "boolean",
            value: true,
          }),
          makeSetting({
            key: "enrollment.legal_agb_document_url",
            type: "text",
            value: "/uploads/enrollment-legal-documents/terms.pdf",
          }),
          makeSetting({
            key: "enrollment.legal_agb_display_mode",
            type: "select",
            value: "pdf",
          }),
        ]}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    fireEvent.click(getByRole("button", { name: "AGB überarbeiten" }));

    expect(getByText("PDF ersetzen")).toBeDefined();
    expect(queryByRole("button", { name: "Entfernen" })).toBeNull();
    expect(
      getByText(
        "Diese PDF kann nicht entfernt werden, solange die AGB aktiv sind und als PDF angezeigt werden. Wechsle zuerst auf Text oder deaktiviere den Block.",
      ),
    ).toBeDefined();
  });

  it("enables AGB terms with an existing PDF source without saving empty text", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const { getByRole } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "enrollment.legal_terms_enabled",
          label: "AGB / Teilnahmebedingungen im Anmeldeformular anzeigen",
          type: "boolean",
          value: false,
        })}
        categoryItems={[
          makeSetting({
            key: "enrollment.legal_agb_text",
            type: "textarea",
            value: "",
          }),
          makeSetting({
            key: "enrollment.legal_agb_document_url",
            type: "text",
            value: "/uploads/enrollment-legal-documents/terms.pdf",
          }),
          makeSetting({
            key: "enrollment.legal_agb_display_mode",
            type: "select",
            value: "text",
          }),
        ]}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    fireEvent.click(getByRole("switch"));
    expect(
      getByRole("heading", {
        name: "AGB / Teilnahmebedingungen aktivieren",
      }),
    ).toBeDefined();
    expect(getByRole("button", { name: "Aktivieren" })).toBeDisabled();

    fireEvent.click(getByRole("button", { name: /PDF-Datei hochladen/ }));
    expect(getByRole("button", { name: "Aktivieren" })).not.toBeDisabled();
    fireEvent.click(getByRole("button", { name: "Aktivieren" }));

    await waitFor(() => {
      expect(onSave).toHaveBeenNthCalledWith(
        1,
        "enrollment.legal_agb_display_mode",
        "pdf",
      );
      expect(onSave).toHaveBeenNthCalledWith(
        2,
        "enrollment.legal_terms_enabled",
        true,
      );
    });
    expect(onSave).not.toHaveBeenCalledWith("enrollment.legal_agb_text", "");
  });

  it("switches AGB editing from PDF source back to text and saves both settings", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const { getByRole, getByLabelText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "enrollment.legal_agb_text",
          label: "AGB / Teilnahmebedingungen",
          type: "textarea",
          value: "Alter AGB-Text",
        })}
        categoryItems={[
          makeSetting({
            key: "enrollment.legal_agb_document_url",
            type: "text",
            value: "/uploads/enrollment-legal-documents/terms.pdf",
          }),
          makeSetting({
            key: "enrollment.legal_agb_display_mode",
            type: "select",
            value: "pdf",
          }),
        ]}
        onSave={onSave}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    fireEvent.click(getByRole("button", { name: "AGB überarbeiten" }));
    fireEvent.click(getByRole("button", { name: /Text eingeben/ }));
    fireEvent.change(getByLabelText("AGB-Text"), {
      target: { value: "Neuer AGB-Text" },
    });
    fireEvent.click(getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(onSave).toHaveBeenNthCalledWith(
        1,
        "enrollment.legal_agb_display_mode",
        "text",
      );
      expect(onSave).toHaveBeenNthCalledWith(
        2,
        "enrollment.legal_agb_text",
        "Neuer AGB-Text",
      );
    });
  });

  it("renders a text field as fallback for unknown type", () => {
    const { container } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          type: "unknown-type" as ResolvedSetting["type"],
          value: "fallback",
        })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    const input = container.querySelector(
      "input[type='text']",
    ) as HTMLInputElement;
    expect(input).not.toBeNull();
    expect(input.value).toBe("fallback");
  });
});
