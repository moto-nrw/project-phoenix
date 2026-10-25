import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, waitFor } from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";
import { SettingsField } from "./settings-field";
import type { ResolvedSetting } from "~/lib/settings-api";

function retentionSetting(
  overrides: Partial<ResolvedSetting> = {},
): ResolvedSetting {
  return {
    key: "gdpr.staff_message_retention_days",
    label: "Team-Chat aufbewahren (Tage)",
    description:
      "Wie lange Nachrichten im internen Team-Chat gespeichert bleiben.",
    type: "number",
    default: 365,
    value: 365,
    is_default: true,
    writable: true,
    visible: true,
    sort_order: 16,
    access_policy: "operator_managed",
    levels: "schule",
    source: "default",
    validation: { min: 30, max: 730 },
    depends_on: null,
    options: null,
    ...overrides,
  };
}

function renderOperatorField(setting: ResolvedSetting) {
  const onSave = vi.fn().mockResolvedValue(null);
  const onReset = vi.fn().mockResolvedValue(null);
  const view = render(
    <ToastProvider>
      <SettingsField
        setting={setting}
        onSave={onSave}
        onReset={onReset}
        audience="operator"
      />
    </ToastProvider>,
  );
  const input = view.container.querySelector(
    "input[type='number']",
  ) as HTMLInputElement;
  return { ...view, input, onSave, onReset };
}

describe("SettingsField retention windows (ADR 0048)", () => {
  it("asks before a shorter retention window is saved", async () => {
    const { input, onSave, getByRole, getByText, findByText } =
      renderOperatorField(retentionSetting());

    fireEvent.change(input, { target: { value: "90" } });
    fireEvent.blur(input);

    expect(await findByText(/älter als 90 Tage sind/)).toBeInTheDocument();
    expect(getByText("Löschfrist verkürzen?")).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();

    fireEvent.click(getByRole("button", { name: "Frist verkürzen" }));
    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(
        "gdpr.staff_message_retention_days",
        90,
      );
    });
  });

  it("keeps the old window when the shortening is cancelled", async () => {
    const { input, onSave, getByRole, findByText } =
      renderOperatorField(retentionSetting());

    fireEvent.change(input, { target: { value: "90" } });
    fireEvent.blur(input);
    await findByText(/älter als 90 Tage sind/);
    fireEvent.click(getByRole("button", { name: "Abbrechen" }));

    await waitFor(() => expect(input.value).toBe("365"));
    expect(onSave).not.toHaveBeenCalled();
  });

  it("does not save an unconfirmed shorter window when the field unmounts", async () => {
    const { input, onSave, findByText, unmount } =
      renderOperatorField(retentionSetting());

    fireEvent.change(input, { target: { value: "90" } });
    fireEvent.blur(input);
    await findByText(/älter als 90 Tage sind/);
    unmount();

    expect(onSave).not.toHaveBeenCalled();
  });

  it("does not save a pending shorter window when the field unmounts", async () => {
    const { input, onSave, unmount } = renderOperatorField(retentionSetting());

    fireEvent.change(input, { target: { value: "90" } });
    unmount();

    await Promise.resolve();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("saves a longer retention window without asking", async () => {
    const { input, onSave, queryByText } =
      renderOperatorField(retentionSetting());

    // Anker: das Zahlenfeld der Frist ist gerendert.
    expect(input).toBeInTheDocument();
    fireEvent.change(input, { target: { value: "500" } });
    fireEvent.blur(input);

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(
        "gdpr.staff_message_retention_days",
        500,
      );
    });
    expect(queryByText("Löschfrist verkürzen?")).toBeNull();
  });

  it("asks before a reset shortens the window to the default", async () => {
    const { container, onReset, getByRole, findByText } = renderOperatorField(
      retentionSetting({ value: 600, is_default: false }),
    );

    fireEvent.click(
      container.querySelector("button[title='Auf Standard zurücksetzen']")!,
    );
    await findByText(/älter als 365 Tage sind/);
    expect(onReset).not.toHaveBeenCalled();

    fireEvent.click(getByRole("button", { name: "Frist verkürzen" }));
    await waitFor(() => {
      expect(onReset).toHaveBeenCalledWith("gdpr.staff_message_retention_days");
    });
  });

  it("tells the school who changes its retention window", () => {
    const { getByText } = render(
      <ToastProvider>
        <SettingsField
          setting={retentionSetting({ writable: false })}
          onSave={vi.fn()}
          onReset={vi.fn()}
        />
      </ToastProvider>,
    );

    expect(getByText("Nur Lesen")).toBeInTheDocument();
    expect(
      getByText(
        "Diese Frist legt moto mit Ihrer Schule fest. Für eine Änderung wenden Sie sich an moto.",
      ),
    ).toBeInTheDocument();
  });

  // #4775: das Bundesland ist keine Frist; der Hinweis nennt, wer es ändert.
  it("tells the school who changes its federal state", () => {
    const { getByText, queryByText } = render(
      <ToastProvider>
        <SettingsField
          setting={retentionSetting({
            key: "operations.federal_state",
            label: "Bundesland",
            description: "Bundesland der Schule.",
            type: "select",
            default: "DE-NW",
            value: "DE-NW",
            validation: null,
            options: {
              static: [{ label: "Nordrhein-Westfalen", value: "DE-NW" }],
            },
            writable: false,
          })}
          onSave={vi.fn()}
          onReset={vi.fn()}
        />
      </ToastProvider>,
    );

    expect(getByText("Nur Lesen")).toBeInTheDocument();
    expect(
      getByText(
        "Das Bundesland legt moto mit Ihrer Schule fest. Stimmt es nicht, wenden Sie sich an das moto-Team.",
      ),
    ).toBeInTheDocument();
    expect(queryByText(/Diese Frist/)).not.toBeInTheDocument();
  });

  it("checks the range before it asks about a shorter window", async () => {
    const { input, onSave, findByText, queryByText } =
      renderOperatorField(retentionSetting());

    fireEvent.change(input, { target: { value: "1" } });
    fireEvent.blur(input);

    expect(await findByText("Minimum: 30")).toBeInTheDocument();
    expect(queryByText("Löschfrist verkürzen?")).toBeNull();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("keeps open parent requests when the parent-message window is shortened", async () => {
    const { input, findByText } = renderOperatorField(
      retentionSetting({
        key: "gdpr.parent_message_retention_days",
        label: "Elternnachrichten aufbewahren (Tage)",
      }),
    );

    fireEvent.change(input, { target: { value: "90" } });
    fireEvent.blur(input);

    expect(
      await findByText(/Offene Elternanfragen bleiben erhalten/),
    ).toBeInTheDocument();
  });
});
