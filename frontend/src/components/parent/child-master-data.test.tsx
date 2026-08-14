import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChildMasterDataView } from "./child-master-data";
import {
  getChildFeatures,
  getChildMasterData,
  submitMasterDataRequest,
  updateMasterDataField,
  type ChildFeatures,
  type ChildMasterData,
} from "~/lib/parent-api";

// The birthday field moved from a native input to the kit picker; this stub
// keeps it readable/settable as an input. Imported inside the factory because
// vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("~/lib/parent-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/parent-api")>();
  return {
    ...actual,
    getChildFeatures: vi.fn(),
    getChildMasterData: vi.fn(),
    getChildCareOfferings: vi.fn().mockResolvedValue({
      period_name: "Schuljahr 2026/27",
      period_start: "2026-08-01",
      period_end: "2027-07-31",
      offerings: [],
      groups: [],
      can_request: false,
      changes_disabled_reason: "no_permission",
    }),
    submitMasterDataRequest: vi.fn(),
    updateMasterDataField: vi.fn(),
  };
});

const mockGetFeatures = vi.mocked(getChildFeatures);
const mockGetMasterData = vi.mocked(getChildMasterData);
const mockSubmit = vi.mocked(submitMasterDataRequest);
const mockUpdateField = vi.mocked(updateMasterDataField);

function features(overrides: Partial<ChildFeatures> = {}): ChildFeatures {
  return {
    sick_note_enabled: true,
    notes_enabled: true,
    excused_requires_approval: false,
    request_submit_enabled: true,
    pickup_change_enabled: true,
    related_accounts_invite_enabled: true,
    related_accounts_remove_enabled: true,
    master_data_edit_enabled: true,
    master_data_contact_edit_enabled: true,
    master_data_request_enabled: true,
    meal_plan_enabled: true,
    has_open_change_request: false,
    parent_news_enabled: true,
    ...overrides,
  };
}

function masterData(overrides: Partial<ChildMasterData> = {}): ChildMasterData {
  return {
    student_id: "42",
    first_name: "Lara",
    last_name: "Beispiel",
    birthday: "2018-03-04",
    school_class: "2a",
    status: "active",
    health_info: "Allergie",
    guardian_profile_id: "77",
    email: "parent@example.test",
    address_street: "Musterweg 1",
    address_city: "Köln",
    address_postal_code: "50667",
    preferred_contact_method: "email",
    language_preference: "de",
    primary_phone: "+491234",
    allowed_departure_modes: {
      mon: ["pickup"],
      tue: ["bus", "alone"],
      wed: [],
    },
    pending_changes: [],
    ...overrides,
  };
}

describe("ChildMasterDataView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetFeatures.mockResolvedValue(features());
    mockGetMasterData.mockResolvedValue(masterData());
    mockUpdateField.mockResolvedValue(masterData({ health_info: "Neue Info" }));
    mockSubmit.mockResolvedValue([]);
  });

  it("loads and renders editable master data with departure matrix", async () => {
    render(<ChildMasterDataView studentId="42" />);

    expect(
      await screen.findByRole("heading", { name: "Daten und Betreuung" }),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue("Lara")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Allergie")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Betreuungszeiten" }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("Wird abgeholt").length).toBeGreaterThan(0);
    // The matrix IS the saved state — every checked box is a stored mode.
    // Fixture: Mo = pickup, Di = bus + alone, Mi = nothing.
    expect(screen.getByLabelText("Mo Wird abgeholt")).toBeChecked();
    expect(screen.getByLabelText("Di Bus")).toBeChecked();
    expect(screen.getByLabelText("Di Geht allein")).toBeChecked();
    expect(screen.getByLabelText("Di Wird abgeholt")).not.toBeChecked();
    expect(screen.getByLabelText("Mi Bus")).not.toBeChecked();
    expect(screen.getByLabelText("Mi Geht allein")).not.toBeChecked();
    expect(screen.getByLabelText("Mi Wird abgeholt")).not.toBeChecked();
    expect(screen.getByRole("link", { name: /Zurück/ })).toHaveAttribute(
      "href",
      "/parents/children/42",
    );
  });

  it("auto-saves direct-edit fields on blur", async () => {
    render(<ChildMasterDataView studentId="42" />);

    const health = await screen.findByDisplayValue("Allergie");
    fireEvent.change(health, { target: { value: "Neue Info" } });
    fireEvent.blur(health);

    await waitFor(() =>
      expect(mockUpdateField).toHaveBeenCalledWith(
        "42",
        "student",
        "health_info",
        "Neue Info",
      ),
    );
    expect(await screen.findByText("Gespeichert")).toBeInTheDocument();
  });

  it("gives editable fields accessible names", async () => {
    render(<ChildMasterDataView studentId="42" />);

    expect(
      await screen.findByLabelText("Gesundheitshinweise / Allergien"),
    ).toHaveValue("Allergie");
    expect(screen.getByLabelText("E-Mail-Adresse")).toHaveValue(
      "parent@example.test",
    );
    expect(
      screen.getByRole("combobox", { name: "Bevorzugter Kontaktweg" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "Sprache" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Vorname")).toHaveValue("Lara");
    expect(screen.getByLabelText("Geburtsdatum")).toHaveValue("2018-03-04");
  });

  it("auto-saves contact method and language controls", async () => {
    render(<ChildMasterDataView studentId="42" />);

    const method = await screen.findByRole("combobox", {
      name: "Bevorzugter Kontaktweg",
    });
    fireEvent.click(method);
    fireEvent.click(screen.getByRole("option", { name: "Telefon" }));

    await waitFor(() =>
      expect(mockUpdateField).toHaveBeenCalledWith(
        "42",
        "guardian_profile",
        "preferred_contact_method",
        "phone",
      ),
    );

    const language = screen.getByRole("combobox", { name: "Sprache" });
    fireEvent.click(language);
    fireEvent.click(screen.getByRole("option", { name: "English" }));

    await waitFor(() =>
      expect(mockUpdateField).toHaveBeenCalledWith(
        "42",
        "guardian_profile",
        "language_preference",
        "en",
      ),
    );
  });

  it("merges direct-save snapshots without rolling back other saved fields", async () => {
    const resolvers: Array<{
      field: string;
      resolve: (value: ChildMasterData) => void;
    }> = [];
    mockUpdateField.mockImplementation(
      (_studentId, _target, field) =>
        new Promise<ChildMasterData>((resolve) => {
          resolvers.push({ field, resolve });
        }),
    );

    render(<ChildMasterDataView studentId="42" />);

    const health = await screen.findByLabelText(
      "Gesundheitshinweise / Allergien",
    );
    const email = screen.getByLabelText("E-Mail-Adresse");

    fireEvent.change(health, { target: { value: "Neue Info" } });
    fireEvent.blur(health);
    fireEvent.change(email, { target: { value: "neu@example.test" } });
    fireEvent.blur(email);

    await waitFor(() => expect(mockUpdateField).toHaveBeenCalledTimes(2));
    expect(resolvers.map((r) => r.field)).toEqual(["health_info", "email"]);

    await act(async () => {
      resolvers[1]!.resolve(
        masterData({
          health_info: "Allergie",
          email: "neu@example.test",
        }),
      );
    });
    expect(email).toHaveValue("neu@example.test");

    await act(async () => {
      resolvers[0]!.resolve(
        masterData({
          health_info: "Neue Info",
          email: "parent@example.test",
        }),
      );
    });

    expect(health).toHaveValue("Neue Info");
    expect(email).toHaveValue("neu@example.test");
  });

  it("does not duplicate a blur save while the debounced save is in flight", async () => {
    let resolveSave: ((value: ChildMasterData) => void) | undefined;
    mockUpdateField.mockImplementation(
      () =>
        new Promise<ChildMasterData>((resolve) => {
          resolveSave = resolve;
        }),
    );

    render(<ChildMasterDataView studentId="42" />);
    const health = await screen.findByDisplayValue("Allergie");

    vi.useFakeTimers();
    try {
      fireEvent.change(health, { target: { value: "Neue Info" } });
      await act(async () => {
        vi.advanceTimersByTime(1500);
      });
      fireEvent.blur(health);

      expect(mockUpdateField).toHaveBeenCalledTimes(1);
      expect(mockUpdateField).toHaveBeenCalledWith(
        "42",
        "student",
        "health_info",
        "Neue Info",
      );

      await act(async () => {
        resolveSave?.(masterData({ health_info: "Neue Info" }));
      });
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps newer autosave input when an older response resolves first", async () => {
    const resolvers: Array<(value: ChildMasterData) => void> = [];
    mockUpdateField.mockImplementation(
      (_studentId, _target, _field, value) =>
        new Promise<ChildMasterData>((resolve) => {
          resolvers.push(() =>
            resolve(masterData({ health_info: String(value) })),
          );
        }),
    );

    render(<ChildMasterDataView studentId="42" />);
    const health = await screen.findByDisplayValue("Allergie");

    vi.useFakeTimers();
    try {
      fireEvent.change(health, { target: { value: "Zwischenstand" } });
      await act(async () => {
        vi.advanceTimersByTime(1500);
      });
      expect(mockUpdateField).toHaveBeenCalledTimes(1);

      fireEvent.change(health, { target: { value: "Neuester Stand" } });
      expect(health).toHaveValue("Neuester Stand");

      await act(async () => {
        resolvers[0]?.(masterData({ health_info: "Zwischenstand" }));
      });

      expect(health).toHaveValue("Neuester Stand");
      expect(mockUpdateField).toHaveBeenCalledTimes(2);
      expect(mockUpdateField).toHaveBeenLastCalledWith(
        "42",
        "student",
        "health_info",
        "Neuester Stand",
      );

      await act(async () => {
        resolvers[1]?.(masterData({ health_info: "Neuester Stand" }));
      });
    } finally {
      vi.useRealTimers();
    }
  });

  it("preserves queued autosave input after an in-flight save fails", async () => {
    let rejectFirst: ((error: Error) => void) | undefined;
    mockUpdateField.mockImplementationOnce(
      () =>
        new Promise<ChildMasterData>((_resolve, reject) => {
          rejectFirst = reject;
        }),
    );
    mockUpdateField.mockImplementationOnce(
      (_studentId, _target, _field, value) =>
        Promise.resolve(masterData({ health_info: String(value) })),
    );

    render(<ChildMasterDataView studentId="42" />);
    const health = await screen.findByDisplayValue("Allergie");

    vi.useFakeTimers();
    try {
      fireEvent.change(health, { target: { value: "Zwischenstand" } });
      await act(async () => {
        vi.advanceTimersByTime(1500);
      });
      expect(mockUpdateField).toHaveBeenCalledTimes(1);

      fireEvent.change(health, { target: { value: "Neuester Stand" } });
      await act(async () => {
        rejectFirst?.(new Error("write failed"));
        await Promise.resolve();
      });

      expect(mockUpdateField).toHaveBeenCalledTimes(2);
      expect(mockUpdateField).toHaveBeenLastCalledWith(
        "42",
        "student",
        "health_info",
        "Neuester Stand",
      );
      expect(health).toHaveValue("Neuester Stand");
    } finally {
      vi.useRealTimers();
    }
  });

  it("submits approval-required identity changes and refreshes pending state", async () => {
    mockGetMasterData.mockResolvedValueOnce(masterData()).mockResolvedValueOnce(
      masterData({
        pending_changes: [
          {
            id: "900",
            target: "person",
            field_key: "first_name",
            old_value: "Lara",
            new_value: "Lea",
            status: "pending",
            created_at: "2026-06-24T12:00:00Z",
          },
        ],
      }),
    );

    render(<ChildMasterDataView studentId="42" />);

    const firstName = await screen.findByDisplayValue("Lara");
    const identityHeading = screen.getByRole("heading", {
      name: "Angaben zum Kind",
    });
    const identitySection = identityHeading.closest("section");
    if (!identitySection) {
      throw new Error("identity section not found");
    }

    fireEvent.change(firstName, { target: { value: "Lea" } });
    fireEvent.click(
      within(identitySection).getByRole("button", {
        name: "Änderung anfragen",
      }),
    );

    await waitFor(() =>
      expect(mockSubmit).toHaveBeenCalledWith("42", [
        { target: "person", field_key: "first_name", value: "Lea" },
      ]),
    );
    expect(
      await screen.findByText(
        "Anfrage gesendet. Das Team prüft Ihre Änderung.",
      ),
    ).toBeInTheDocument();
    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(await screen.findByDisplayValue("Lara")).toBeDisabled();

    fireEvent.click(
      within(identitySection).getByRole("button", {
        name: "Änderung anfragen",
      }),
    );
    expect(mockSubmit).toHaveBeenCalledTimes(1);
  });

  it("keeps identity request success when the pending refresh fails", async () => {
    mockSubmit.mockResolvedValueOnce([
      {
        id: "900",
        target: "person",
        field_key: "first_name",
        old_value: "Lara",
        new_value: "Lea",
        status: "pending",
        created_at: "2026-06-24T12:00:00Z",
      },
    ]);
    mockGetMasterData
      .mockResolvedValueOnce(masterData())
      .mockRejectedValueOnce(new Error("refresh failed"));

    render(<ChildMasterDataView studentId="42" />);

    const firstName = await screen.findByLabelText("Vorname");
    const identityHeading = screen.getByRole("heading", {
      name: "Angaben zum Kind",
    });
    const identitySection = identityHeading.closest("section");
    if (!identitySection) {
      throw new Error("identity section not found");
    }

    fireEvent.change(firstName, { target: { value: "Lea" } });
    fireEvent.click(
      within(identitySection).getByRole("button", {
        name: "Änderung anfragen",
      }),
    );

    expect(
      await screen.findByText(
        "Anfrage gesendet. Das Team prüft Ihre Änderung.",
      ),
    ).toBeInTheDocument();
    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(screen.getByLabelText("Vorname")).toBeDisabled();
    expect(
      screen.queryByText("Die Anfrage konnte nicht gesendet werden."),
    ).not.toBeInTheDocument();
  });

  it("submits departure mode changes for review", async () => {
    mockGetMasterData.mockResolvedValueOnce(masterData()).mockResolvedValueOnce(
      masterData({
        pending_changes: [
          {
            id: "901",
            target: "departure",
            field_key: "allowed_departure_modes",
            old_value: { mon: ["pickup"], tue: ["bus", "alone"] },
            new_value: {
              mon: ["pickup"],
              tue: ["bus", "alone"],
              wed: ["pickup"],
            },
            status: "pending",
            created_at: "2026-06-24T12:00:00Z",
          },
        ],
      }),
    );

    render(<ChildMasterDataView studentId="42" />);

    const departureSection = await screen.findByRole("heading", {
      name: "Dauerhafte Gehzeiten",
    });
    const section = departureSection.closest("section");
    if (!section) {
      throw new Error("departure section not found");
    }

    const departureCheckbox = screen.getByLabelText("Mi Wird abgeholt");
    fireEvent.click(departureCheckbox.nextElementSibling as HTMLElement);
    fireEvent.click(
      within(section).getByRole("button", { name: "Änderung anfragen" }),
    );

    await waitFor(() =>
      expect(mockSubmit).toHaveBeenCalledWith("42", [
        {
          target: "departure",
          field_key: "allowed_departure_modes",
          value: {
            mon: ["pickup"],
            tue: ["alone", "bus"],
            wed: ["pickup"],
          },
        },
      ]),
    );
  });

  it("keeps departure request success when the pending refresh fails", async () => {
    mockSubmit.mockResolvedValueOnce([
      {
        id: "901",
        target: "departure",
        field_key: "allowed_departure_modes",
        old_value: { mon: ["pickup"] },
        new_value: { mon: ["pickup"], wed: ["pickup"] },
        status: "pending",
        created_at: "2026-06-24T12:00:00Z",
      },
    ]);
    mockGetMasterData
      .mockResolvedValueOnce(masterData())
      .mockRejectedValueOnce(new Error("refresh failed"));

    render(<ChildMasterDataView studentId="42" />);

    const departureSection = await screen.findByRole("heading", {
      name: "Dauerhafte Gehzeiten",
    });
    const section = departureSection.closest("section");
    if (!section) {
      throw new Error("departure section not found");
    }

    fireEvent.click(screen.getByLabelText("Mi Wird abgeholt"));
    fireEvent.click(
      within(section).getByRole("button", { name: "Änderung anfragen" }),
    );

    expect(
      await screen.findByText(
        "Anfrage gesendet. Das Team prüft Ihre Änderung.",
      ),
    ).toBeInTheDocument();
    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(
      within(section).getByRole("button", { name: "Änderung anfragen" }),
    ).toBeDisabled();
    expect(
      screen.queryByText("Die Anfrage konnte nicht gesendet werden."),
    ).not.toBeInTheDocument();
  });

  it("preserves dirty request drafts across unrelated direct-edit refreshes", async () => {
    mockUpdateField.mockResolvedValue(masterData({ health_info: "Neue Info" }));

    render(<ChildMasterDataView studentId="42" />);

    const firstName = await screen.findByDisplayValue("Lara");
    fireEvent.change(firstName, { target: { value: "Lea" } });

    const health = screen.getByDisplayValue("Allergie");
    fireEvent.change(health, { target: { value: "Neue Info" } });
    fireEvent.blur(health);

    await waitFor(() => expect(mockUpdateField).toHaveBeenCalled());
    expect(firstName).toHaveValue("Lea");
  });

  it("preserves dirty departure drafts across unrelated direct-edit refreshes", async () => {
    mockUpdateField.mockResolvedValue(masterData({ health_info: "Neue Info" }));

    render(<ChildMasterDataView studentId="42" />);

    await screen.findByRole("heading", { name: "Dauerhafte Gehzeiten" });
    const wedPickup = screen.getByLabelText("Mi Wird abgeholt");
    fireEvent.click(wedPickup);
    expect(wedPickup).toBeChecked();

    const health = screen.getByDisplayValue("Allergie");
    fireEvent.change(health, { target: { value: "Neue Info" } });
    fireEvent.blur(health);

    await waitFor(() => expect(mockUpdateField).toHaveBeenCalled());
    expect(wedPickup).toBeChecked();
  });

  it("does not offer accompanied departure requests without companion-note support", async () => {
    mockGetMasterData.mockResolvedValue(
      masterData({
        allowed_departure_modes: {
          mon: ["accompanied"],
        },
      }),
    );

    render(<ChildMasterDataView studentId="42" />);

    const departureSection = await screen.findByRole("heading", {
      name: "Dauerhafte Gehzeiten",
    });
    const section = departureSection.closest("section");
    if (!section) {
      throw new Error("departure section not found");
    }

    expect(
      screen.queryByLabelText("Mo Mit anderem Kind"),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Mo Geht allein")).toBeDisabled();
    expect(
      within(section).getByRole("button", { name: "Änderung anfragen" }),
    ).toBeDisabled();
  });

  it("shows disabled messaging when feature flags are off", async () => {
    mockGetFeatures.mockResolvedValue(
      features({
        master_data_edit_enabled: false,
        master_data_contact_edit_enabled: false,
        master_data_request_enabled: false,
      }),
    );

    render(<ChildMasterDataView studentId="42" />);

    await screen.findAllByText(
      "Änderungsanfragen sind bei dieser OGS deaktiviert.",
    );
    expect(
      screen.getByText(
        "Das Bearbeiten der Stammdaten ist bei dieser OGS deaktiviert.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue("Allergie")).toBeDisabled();
  });

  it("keeps health editable but disables contact fields when contact edits are off", async () => {
    mockGetFeatures.mockResolvedValue(
      features({ master_data_contact_edit_enabled: false }),
    );

    render(<ChildMasterDataView studentId="42" />);

    expect(await screen.findByDisplayValue("Allergie")).not.toBeDisabled();
    expect(screen.getByDisplayValue("parent@example.test")).toBeDisabled();
    expect(screen.getByDisplayValue("+491234")).toBeDisabled();
    expect(
      screen.getByRole("combobox", { name: "Bevorzugter Kontaktweg" }),
    ).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "Sprache" })).toBeDisabled();
    expect(screen.getByDisplayValue("Musterweg 1")).toBeDisabled();
    expect(
      screen.getByText(
        "Das Bearbeiten der Stammdaten ist bei dieser OGS deaktiviert.",
      ),
    ).toBeInTheDocument();
  });

  it("renders a load error if either request fails", async () => {
    mockGetMasterData.mockRejectedValue(new Error("boom"));

    render(<ChildMasterDataView studentId="42" />);

    expect(
      await screen.findByText(
        "Die Stammdaten konnten nicht geladen werden. Bitte aktualisieren Sie die Seite.",
      ),
    ).toBeInTheDocument();
  });

  it("shows direct-save and request-submit errors", async () => {
    mockUpdateField.mockRejectedValueOnce(new Error("write failed"));
    mockSubmit.mockRejectedValueOnce(new Error("request failed"));

    render(<ChildMasterDataView studentId="42" />);

    const health = await screen.findByDisplayValue("Allergie");
    fireEvent.change(health, { target: { value: "Neue Info" } });
    fireEvent.blur(health);
    expect(
      await screen.findByText("Die Änderung konnte nicht gespeichert werden."),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByDisplayValue("Lara"), {
      target: { value: "Lea" },
    });
    const identityHeading = screen.getByRole("heading", {
      name: "Angaben zum Kind",
    });
    const identitySection = identityHeading.closest("section");
    if (!identitySection) {
      throw new Error("identity section not found");
    }
    fireEvent.click(
      within(identitySection).getByRole("button", {
        name: "Änderung anfragen",
      }),
    );

    expect(
      await screen.findByText("Die Anfrage konnte nicht gesendet werden."),
    ).toBeInTheDocument();
  });
});
