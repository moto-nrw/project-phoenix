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
  const visibilityKey = "operations.operational_overview_scope";
  const attendanceKey = "operations.attendance_edit_scope";

  function scopeSetting(key: string, value: string, writable = true) {
    return makeSetting({
      key,
      type: "select",
      value,
      writable,
      options: {
        static: [
          { value: "own", label: "Eigene Zuständigkeiten" },
          { value: "all_staff", label: "Alle Gruppen und Blöcke" },
        ],
      },
    });
  }

  it.each([
    [attendanceKey, visibilityKey, "own", "all_staff", "Beides erweitern"],
    [visibilityKey, attendanceKey, "all_staff", "own", "Beides begrenzen"],
  ])(
    "confirms and orders paired changes for %s",
    async (key, siblingKey, current, next, confirmText) => {
      const onSave = vi.fn().mockResolvedValue(null);
      const setting = scopeSetting(key, current);
      const { getByRole } = renderWithProviders(
        <SettingsField
          setting={setting}
          categoryItems={[setting, scopeSetting(siblingKey, current)]}
          onSave={onSave}
          onReset={vi.fn()}
        />,
      );
      fireEvent.click(getByRole("combobox"));
      fireEvent.click(
        getByRole("option", {
          name:
            next === "own"
              ? "Eigene Zuständigkeiten"
              : "Alle Gruppen und Blöcke",
        }),
      );
      expect(onSave).not.toHaveBeenCalled();
      fireEvent.click(getByRole("button", { name: confirmText }));
      await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
      expect(onSave).toHaveBeenNthCalledWith(1, siblingKey, next);
      expect(onSave).toHaveBeenNthCalledWith(2, key, next);
    },
  );

  it("cancels paired changes without saving either setting", () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const setting = scopeSetting(attendanceKey, "own");
    const { getByRole, queryByRole } = renderWithProviders(
      <SettingsField
        setting={setting}
        categoryItems={[setting, scopeSetting(visibilityKey, "own")]}
        onSave={onSave}
        onReset={vi.fn()}
      />,
    );
    fireEvent.click(getByRole("combobox"));
    fireEvent.click(getByRole("option", { name: "Alle Gruppen und Blöcke" }));
    fireEvent.click(getByRole("button", { name: "Abbrechen" }));
    expect(onSave).not.toHaveBeenCalled();
    expect(getByRole("combobox")).toHaveTextContent("Eigene Zuständigkeiten");
    expect(queryByRole("button", { name: "Beides erweitern" })).toBeNull();
  });

  it("does not expand editing when saving the visibility prerequisite fails", async () => {
    const onSave = vi.fn().mockResolvedValue("Speichern fehlgeschlagen.");
    const setting = scopeSetting(attendanceKey, "own");
    const { getByRole } = renderWithProviders(
      <SettingsField
        setting={setting}
        categoryItems={[setting, scopeSetting(visibilityKey, "own")]}
        onSave={onSave}
        onReset={vi.fn()}
      />,
    );
    fireEvent.click(getByRole("combobox"));
    fireEvent.click(getByRole("option", { name: "Alle Gruppen und Blöcke" }));
    fireEvent.click(getByRole("button", { name: "Beides erweitern" }));
    await waitFor(() =>
      expect(document.body.textContent).toContain("Speichern fehlgeschlagen."),
    );
    expect(onSave).toHaveBeenCalledExactlyOnceWith(visibilityKey, "all_staff");
  });

  it("keeps the approved visibility change when the second write fails", async () => {
    const onSave = vi
      .fn()
      .mockResolvedValueOnce(null)
      .mockResolvedValueOnce("Bearbeitung konnte nicht gespeichert werden.");
    const setting = scopeSetting(attendanceKey, "own");
    const { getByRole } = renderWithProviders(
      <SettingsField
        setting={setting}
        categoryItems={[setting, scopeSetting(visibilityKey, "own")]}
        onSave={onSave}
        onReset={vi.fn()}
      />,
    );
    fireEvent.click(getByRole("combobox"));
    fireEvent.click(getByRole("option", { name: "Alle Gruppen und Blöcke" }));
    fireEvent.click(getByRole("button", { name: "Beides erweitern" }));
    await waitFor(() =>
      expect(document.body.textContent).toContain(
        "Bearbeitung konnte nicht gespeichert werden.",
      ),
    );
    expect(onSave.mock.calls).toEqual([
      [visibilityKey, "all_staff"],
      [attendanceKey, "all_staff"],
    ]);
    expect(getByRole("combobox")).toHaveTextContent("Eigene Zuständigkeiten");
  });

  it("saves attendance scope directly when visibility already permits it", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const setting = scopeSetting(attendanceKey, "own");
    const { getByRole, queryByRole } = renderWithProviders(
      <SettingsField
        setting={setting}
        categoryItems={[setting, scopeSetting(visibilityKey, "all_staff")]}
        onSave={onSave}
        onReset={vi.fn()}
      />,
    );
    fireEvent.click(getByRole("combobox"));
    fireEvent.click(getByRole("option", { name: "Alle Gruppen und Blöcke" }));
    await waitFor(() =>
      expect(onSave).toHaveBeenCalledExactlyOnceWith(
        attendanceKey,
        "all_staff",
      ),
    );
    expect(queryByRole("button", { name: "Beides erweitern" })).toBeNull();
  });

  it("does not offer a paired write for a read-only prerequisite", () => {
    const onSave = vi.fn();
    const setting = scopeSetting(attendanceKey, "own");
    const { getByRole, queryByRole } = renderWithProviders(
      <SettingsField
        setting={setting}
        categoryItems={[setting, scopeSetting(visibilityKey, "own", false)]}
        onSave={onSave}
        onReset={vi.fn()}
      />,
    );
    fireEvent.click(getByRole("combobox"));
    fireEvent.click(getByRole("option", { name: "Alle Gruppen und Blöcke" }));
    expect(onSave).not.toHaveBeenCalled();
    expect(queryByRole("button", { name: "Beides erweitern" })).toBeNull();
    expect(document.body.textContent).toContain(
      "Dafür muss zuerst die andere Einstellung geändert werden.",
    );
  });

  const startKey = "operations.block_start_scope";
  const endKey = "operations.block_complete_scope";

  function pickOption(
    getByRole: ReturnType<typeof renderWithProviders>["getByRole"],
    name: string,
  ) {
    fireEvent.click(getByRole("combobox"));
    fireEvent.click(getByRole("option", { name }));
  }

  it.each([startKey, endKey])(
    "expands visibility before the whole team may use %s",
    async (key) => {
      const onSave = vi.fn().mockResolvedValue(null);
      const setting = scopeSetting(key, "own");
      const { getByRole, getByText } = renderWithProviders(
        <SettingsField
          setting={setting}
          categoryItems={[setting, scopeSetting(visibilityKey, "own")]}
          onSave={onSave}
          onReset={vi.fn()}
        />,
      );
      pickOption(getByRole, "Alle Gruppen und Blöcke");
      expect(onSave).not.toHaveBeenCalled();
      expect(getByText("Auch den Sichtbereich erweitern?")).toBeInTheDocument();
      fireEvent.click(getByRole("button", { name: "Beides erweitern" }));
      await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
      expect(onSave.mock.calls).toEqual([
        [visibilityKey, "all_staff"],
        [key, "all_staff"],
      ]);
    },
  );

  it.each([
    ["ending only", "own", "own", "all_staff", [endKey], "Beides begrenzen"],
    [
      "all three",
      "all_staff",
      "all_staff",
      "all_staff",
      [attendanceKey, startKey, endKey],
      "Alles begrenzen",
    ],
  ])(
    "restricts %s before visibility",
    async (_name, attendance, starting, ending, restricted, confirmText) => {
      const onSave = vi.fn().mockResolvedValue(null);
      const setting = scopeSetting(visibilityKey, "all_staff");
      const { getByRole } = renderWithProviders(
        <SettingsField
          setting={setting}
          categoryItems={[
            setting,
            scopeSetting(attendanceKey, attendance),
            scopeSetting(startKey, starting),
            scopeSetting(endKey, ending),
          ]}
          onSave={onSave}
          onReset={vi.fn()}
        />,
      );
      pickOption(getByRole, "Eigene Zuständigkeiten");
      fireEvent.click(getByRole("button", { name: confirmText }));
      await waitFor(() =>
        expect(onSave).toHaveBeenCalledTimes(restricted.length + 1),
      );
      expect(onSave.mock.calls).toEqual([
        ...restricted.map((key) => [key, "own"]),
        [visibilityKey, "own"],
      ]);
    },
  );

  it("expands visibility before the whole team may start blocks", async () => {
    const onSave = vi.fn().mockResolvedValue(null);
    const setting = scopeSetting(startKey, "own");
    const { getByRole, getByText } = renderWithProviders(
      <SettingsField
        setting={setting}
        categoryItems={[
          setting,
          scopeSetting(visibilityKey, "own"),
          scopeSetting(attendanceKey, "own"),
        ]}
        onSave={onSave}
        onReset={vi.fn()}
      />,
    );
    pickOption(getByRole, "Alle Gruppen und Blöcke");
    expect(onSave).not.toHaveBeenCalled();
    expect(getByText("Auch den Sichtbereich erweitern?")).toBeInTheDocument();
    fireEvent.click(getByRole("button", { name: "Beides erweitern" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
    expect(onSave.mock.calls).toEqual([
      [visibilityKey, "all_staff"],
      [startKey, "all_staff"],
    ]);
  });

  it.each([
    ["starting only", "own", "all_staff", [startKey], "Beides begrenzen"],
    [
      "attendance and starting",
      "all_staff",
      "all_staff",
      [attendanceKey, startKey],
      "Alles begrenzen",
    ],
  ])(
    "restricts %s before visibility",
    async (_name, attendance, starting, restricted, confirmText) => {
      const onSave = vi.fn().mockResolvedValue(null);
      const setting = scopeSetting(visibilityKey, "all_staff");
      const { getByRole } = renderWithProviders(
        <SettingsField
          setting={setting}
          categoryItems={[
            setting,
            scopeSetting(attendanceKey, attendance),
            scopeSetting(startKey, starting),
          ]}
          onSave={onSave}
          onReset={vi.fn()}
        />,
      );
      pickOption(getByRole, "Eigene Zuständigkeiten");
      expect(onSave).not.toHaveBeenCalled();
      fireEvent.click(getByRole("button", { name: confirmText }));
      await waitFor(() =>
        expect(onSave).toHaveBeenCalledTimes(restricted.length + 1),
      );
      expect(onSave.mock.calls).toEqual([
        ...restricted.map((key) => [key, "own"]),
        [visibilityKey, "own"],
      ]);
    },
  );

  it("stops the restriction at the first failed dependent write", async () => {
    const onSave = vi.fn().mockResolvedValue("Speichern fehlgeschlagen.");
    const setting = scopeSetting(visibilityKey, "all_staff");
    const { getByRole } = renderWithProviders(
      <SettingsField
        setting={setting}
        categoryItems={[
          setting,
          scopeSetting(attendanceKey, "all_staff"),
          scopeSetting(startKey, "all_staff"),
        ]}
        onSave={onSave}
        onReset={vi.fn()}
      />,
    );
    pickOption(getByRole, "Eigene Zuständigkeiten");
    fireEvent.click(getByRole("button", { name: "Alles begrenzen" }));
    await waitFor(() =>
      expect(document.body.textContent).toContain("Speichern fehlgeschlagen."),
    );
    expect(onSave).toHaveBeenCalledExactlyOnceWith(attendanceKey, "own");
  });

  it("does not restrict visibility when the start setting is read-only", () => {
    const onSave = vi.fn();
    const setting = scopeSetting(visibilityKey, "all_staff");
    const { getByRole } = renderWithProviders(
      <SettingsField
        setting={setting}
        categoryItems={[
          setting,
          scopeSetting(attendanceKey, "own"),
          scopeSetting(startKey, "all_staff", false),
        ]}
        onSave={onSave}
        onReset={vi.fn()}
      />,
    );
    pickOption(getByRole, "Eigene Zuständigkeiten");
    expect(onSave).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain(
      "Dafür muss zuerst die andere Einstellung geändert werden.",
    );
  });

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

  // #3372: a lesson without an end time is not maintained; „Jederzeit“ would
  // claim the lesson ends at any time.
  it("shows an empty lesson end time as not entered", () => {
    const { getByText, queryByText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "school_periods.end_5",
          type: "time",
          value: "",
          default: "",
        })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Nicht eingetragen")).toBeInTheDocument();
    expect(queryByText("Jederzeit")).not.toBeInTheDocument();
  });

  // #3371: an empty care time preset offers nothing; „Jederzeit“ would read as
  // „children may arrive at any time“.
  it("shows an empty care time preset as not entered", () => {
    const { getByText, queryByText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "care_times.default_pickup",
          type: "time",
          value: "",
          default: "",
        })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Nicht eingetragen")).toBeInTheDocument();
    expect(queryByText("Jederzeit")).not.toBeInTheDocument();
  });

  it("keeps Jederzeit for other optional time settings", () => {
    const { getByText } = renderWithProviders(
      <SettingsField
        setting={makeSetting({
          key: "checkout.student_daily_checkout_time",
          type: "time",
          value: "",
          default: "",
        })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Jederzeit")).toBeInTheDocument();
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
      const errorText = container.querySelector(".text-moto-red");
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
      const errorText = container.querySelector(".text-moto-red");
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
