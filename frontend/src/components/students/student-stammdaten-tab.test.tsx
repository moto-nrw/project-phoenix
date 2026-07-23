import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  act,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";

const {
  handleStudentFormSubmitMock,
  validateStudentFormMock,
  uploadStudentPhotoMock,
  deleteStudentPhotoMock,
  fetchStudentPrivacyConsentMock,
  fetchStudentEnrollmentExtraFieldsMock,
  fetchStudentCompanionsMock,
} = vi.hoisted(() => ({
  handleStudentFormSubmitMock: vi.fn(),
  validateStudentFormMock: vi.fn(),
  uploadStudentPhotoMock: vi.fn(),
  deleteStudentPhotoMock: vi.fn(),
  fetchStudentPrivacyConsentMock: vi.fn<
    () => Promise<{ accepted: boolean; dataRetentionDays: number } | null>
  >(() => Promise.resolve(null)),
  fetchStudentEnrollmentExtraFieldsMock: vi.fn<
    (studentId: string) => Promise<unknown[]>
  >(() => Promise.resolve([])),
  // Unreachable by default, exactly like the un-mocked network call these
  // tests used to make: the tab then leaves the companion list alone instead
  // of submitting one, which is what every suite below assumes.
  fetchStudentCompanionsMock: vi.fn<(studentId: string) => Promise<unknown[]>>(
    () => Promise.reject(new Error("companions unavailable")),
  ),
}));

vi.mock("~/lib/student-companion-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-companion-api")>()),
  fetchStudentCompanions: fetchStudentCompanionsMock,
}));

vi.mock("~/lib/student-form-validation", () => ({
  handleStudentFormSubmit: handleStudentFormSubmitMock,
  validateStudentForm: validateStudentFormMock,
}));

vi.mock("~/lib/student-api", () => ({
  uploadStudentPhoto: uploadStudentPhotoMock,
  deleteStudentPhoto: deleteStudentPhotoMock,
  fetchStudentPrivacyConsent: fetchStudentPrivacyConsentMock,
  fetchStudentEnrollmentExtraFields: fetchStudentEnrollmentExtraFieldsMock,
}));

// Mock StudentPhotoSection with controllable test buttons. The real component
// has file pickers, image compression, and consent banners — none of which
// matter for testing the parent's submit-orchestration logic. This stub
// exposes test-ids that drive the parent's pending-photo handlers directly.
vi.mock("./student-photo-section", () => ({
  StudentPhotoSection: ({
    consentGiven,
    onConsentChange,
    onPickPhoto,
    onMarkRemoved,
    onCancelRemove,
  }: {
    consentGiven: boolean;
    onConsentChange: (value: boolean) => void;
    onPickPhoto: (blob: Blob | null) => void;
    onMarkRemoved: () => void;
    onCancelRemove: () => void;
  }) => (
    <div data-testid="photo-section" data-consent={String(consentGiven)}>
      <button
        type="button"
        data-testid="pick-photo"
        onClick={() =>
          onPickPhoto(new Blob(["fake-image"], { type: "image/jpeg" }))
        }
      >
        pick
      </button>
      <button
        type="button"
        data-testid="consent-on"
        onClick={() => onConsentChange(true)}
      >
        consent-on
      </button>
      <button
        type="button"
        data-testid="consent-off"
        onClick={() => onConsentChange(false)}
      >
        consent-off
      </button>
      <button
        type="button"
        data-testid="mark-removed"
        onClick={() => onMarkRemoved()}
      >
        remove
      </button>
      <button
        type="button"
        data-testid="cancel-remove"
        onClick={() => onCancelRemove()}
      >
        cancel-remove
      </button>
    </div>
  ),
}));

vi.mock("./student-form-fields", () => ({
  PersonalInfoSection: ({
    formData,
    onChange,
  }: {
    formData: Partial<Student>;
    onChange: (field: keyof Student, value: string | boolean | number) => void;
  }) => (
    <div data-testid="personal-info">
      <input
        data-testid="first-name"
        value={formData.first_name ?? ""}
        onChange={(e) => onChange("first_name", e.target.value)}
      />
      <input
        data-testid="address-street"
        value={formData.address_street ?? ""}
        onChange={(e) => onChange("address_street", e.target.value)}
      />
      <input
        data-testid="address-postal-code"
        value={formData.address_postal_code ?? ""}
        onChange={(e) => onChange("address_postal_code", e.target.value)}
      />
      <input
        data-testid="address-city"
        value={formData.address_city ?? ""}
        onChange={(e) => onChange("address_city", e.target.value)}
      />
    </div>
  ),
  DepartureSection: ({
    days,
    onChange,
    onCompanionsChange,
  }: {
    days?: Record<string, string[]> | null;
    onChange: (v: Record<string, string[]>) => void;
    onCompanionsChange?: (
      companions: { companion_student_id: string; weekdays: string[] }[],
    ) => void;
  }) => (
    <div data-testid="departure-section">
      <span data-testid="departure-mon">{days?.mon ?? "alone"}</span>
      <button
        type="button"
        data-testid="departure-set-mon-bus"
        onClick={() => onChange({ ...days, mon: ["bus"] })}
      >
        set-mon-bus
      </button>
      {onCompanionsChange ? (
        <button
          type="button"
          data-testid="companion-add"
          onClick={() =>
            onCompanionsChange([
              { companion_student_id: "7", weekdays: ["mon"] },
            ])
          }
        >
          add-companion
        </button>
      ) : null}
    </div>
  ),
  // EnrollmentConsentsSection: read-only consent display rendered
  // below the student form. The tests don't exercise it but the mock
  // must satisfy the import in student-stammdaten-tab.tsx.
  EnrollmentConsentsSection: () => <div data-testid="enrollment-consents" />,
}));

vi.mock("./student-common-form-sections", () => ({
  StudentCommonFormSections: ({
    formData,
    errors,
    onChange,
  }: {
    formData: Partial<Student>;
    errors: Record<string, string>;
    onChange: (field: keyof Student, value: string | boolean | number) => void;
  }) => (
    <div
      data-testid="common-sections"
      data-error-count={Object.keys(errors).length}
      // The separately fetched consent reaches the draft one effect later than
      // the fetch resolves; the tests wait on these before editing, because an
      // edit made in between is overwritten by that sync.
      data-privacy-consent={String(formData.privacy_consent_accepted)}
      data-retention-days={String(formData.data_retention_days)}
    >
      {/* The Datenschutz controls live in this section; the tests drive them
          directly to tell a consent edit apart from an unrelated save. */}
      <button
        type="button"
        data-testid="privacy-consent-on"
        onClick={() => onChange("privacy_consent_accepted", true)}
      >
        privacy-consent-on
      </button>
      <button
        type="button"
        data-testid="retention-90"
        onClick={() => onChange("data_retention_days", 90)}
      >
        retention-90
      </button>
    </div>
  ),
}));

vi.mock("~/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: string;
  }) => (
    <button type="button" {...props}>
      {children}
    </button>
  ),
}));

import { StudentStammdatenTab } from "./student-stammdaten-tab";
import { notifyStudentCompanionsChanged } from "~/lib/student-companion-api";

function makeStudent(overrides: Partial<Student> = {}): Student {
  return {
    id: "1",
    name: "Max Muster",
    first_name: "Max",
    second_name: "Muster",
    school_class: "3a",
    current_location: "class",
    ...overrides,
  } as Student;
}

/**
 * Blocks until the separately fetched Datenschutz values are in the editable
 * draft. Editing before that is pointless: the sync effect writes the server
 * values over the whole pair and takes the edit with it.
 *
 * At least one of the awaited values has to differ from the pre-load draft
 * (not accepted / 30 days), or the wait passes before the fetch resolved.
 */
async function waitForConsentInDraft(accepted: string, retentionDays: string) {
  await waitFor(() => {
    const section = screen.getByTestId("common-sections");
    expect(section).toHaveAttribute("data-privacy-consent", accepted);
    expect(section).toHaveAttribute("data-retention-days", retentionDays);
  });
}

describe("StudentStammdatenTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchStudentPrivacyConsentMock.mockResolvedValue(null);
    fetchStudentEnrollmentExtraFieldsMock.mockResolvedValue([]);
    validateStudentFormMock.mockReturnValue({});
    handleStudentFormSubmitMock.mockImplementation(
      async (
        event: Event & { preventDefault: () => void },
        _formData: Partial<Student>,
        validate: () => boolean,
        save: (data: Partial<Student>) => Promise<void>,
        setSaving: (v: boolean) => void,
        setErrors: (e: Record<string, string>) => void,
      ) => {
        event.preventDefault();
        if (!validate()) return;
        setSaving(true);
        try {
          await save(_formData);
        } catch (err) {
          setErrors({ submit: err instanceof Error ? err.message : "err" });
        } finally {
          setSaving(false);
        }
      },
    );
  });

  it("submit button is disabled when form is unchanged", () => {
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /Speichern/ })).toBeDisabled();
  });

  it("enables submit once a field changes", () => {
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "New" },
    });

    expect(screen.getByRole("button", { name: /Speichern/ })).toBeEnabled();
  });

  it("renders persisted address fields in the editable draft", () => {
    render(
      <StudentStammdatenTab
        student={makeStudent({
          address_street: "Musterstraße 12",
          address_postal_code: "50667",
          address_city: "Köln",
        })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect(screen.getByTestId<HTMLInputElement>("address-street").value).toBe(
      "Musterstraße 12",
    );
    expect(
      screen.getByTestId<HTMLInputElement>("address-postal-code").value,
    ).toBe("50667");
    expect(screen.getByTestId<HTMLInputElement>("address-city").value).toBe(
      "Köln",
    );
  });

  it("enables submit and persists address-only edits", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={makeStudent({ address_city: "Köln" })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("address-city"), {
      target: { value: "Bonn" },
    });

    const saveButton = screen.getByRole("button", { name: /Speichern/ });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(
        expect.objectContaining({ address_city: "Bonn" }),
      );
    });
  });

  // The proxy writes the Datenschutz pair BEFORE the student PUT, so anything
  // this form sends is stored even when the student write is refused. An
  // unrelated edit must therefore not carry the loaded consent along: it would
  // overwrite a change somebody else made since the load and survive the
  // refusal.
  it("omits the unchanged Datenschutz pair from an unrelated save", async () => {
    fetchStudentPrivacyConsentMock.mockResolvedValue({
      accepted: true,
      dataRetentionDays: 90,
    });
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={makeStudent({ address_city: "Köln" })}
        groups={[]}
        onSave={onSave}
      />,
    );

    // Wait for the fetched consent to land in the draft — before that there is
    // nothing to omit and the assertion would pass for the wrong reason.
    await waitForConsentInDraft("true", "90");

    fireEvent.change(screen.getByTestId("address-city"), {
      target: { value: "Bonn" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(submitted.address_city).toBe("Bonn");
    expect("privacy_consent_accepted" in submitted).toBe(false);
    expect("data_retention_days" in submitted).toBe(false);
  });

  // Both fields travel together: the proxy's consent PUT upserts the pair, so a
  // lone field would reset the other to its default.
  it("submits both Datenschutz fields when one of them changed", async () => {
    // 60 days, not the 30 the draft starts with: the wait below would otherwise
    // be satisfied by the pre-load defaults and the edit would race the sync.
    fetchStudentPrivacyConsentMock.mockResolvedValue({
      accepted: false,
      dataRetentionDays: 60,
    });
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    await waitForConsentInDraft("false", "60");

    fireEvent.click(screen.getByTestId("privacy-consent-on"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(submitted.privacy_consent_accepted).toBe(true);
    expect(submitted.data_retention_days).toBe(60);
  });

  it("submits both Datenschutz fields when only the retention changed", async () => {
    fetchStudentPrivacyConsentMock.mockResolvedValue({
      accepted: true,
      dataRetentionDays: 30,
    });
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    await waitForConsentInDraft("true", "30");

    fireEvent.click(screen.getByTestId("retention-90"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(submitted.data_retention_days).toBe(90);
    expect(submitted.privacy_consent_accepted).toBe(true);
  });

  it("renders linked per-child enrollment extra fields read-only", async () => {
    fetchStudentEnrollmentExtraFieldsMock.mockResolvedValue([
      {
        request_id: "77",
        phase_name: "Anmeldung 2026",
        submitted_at: "2026-06-01T12:00:00Z",
        fields: [
          {
            key: "swimming_level",
            label: "Schwimmfähigkeit",
            type: "select",
            options: [{ label: "Kann sicher schwimmen", value: "safe" }],
            value: "safe",
          },
        ],
      },
    ]);

    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect(await screen.findByText("Zusatzangaben")).toBeInTheDocument();
    expect(screen.getByText("Schwimmfähigkeit")).toBeInTheDocument();
    expect(screen.getByText("Kann sicher schwimmen")).toBeInTheDocument();
  });

  it("clears previous enrollment extra fields while loading a different student", async () => {
    fetchStudentEnrollmentExtraFieldsMock.mockImplementation(
      (studentId: string) => {
        if (studentId === "1") {
          return Promise.resolve([
            {
              request_id: "77",
              phase_name: "Anmeldung 2026",
              submitted_at: "2026-06-01T12:00:00Z",
              fields: [
                {
                  key: "swimming_level",
                  label: "Schwimmfähigkeit",
                  type: "select",
                  options: [{ label: "Kann sicher schwimmen", value: "safe" }],
                  value: "safe",
                },
              ],
            },
          ]);
        }
        return new Promise(() => {
          // Keep the second request in flight so stale data would remain visible.
        });
      },
    );

    const { rerender } = render(
      <StudentStammdatenTab
        student={makeStudent({ id: "1", first_name: "Max" })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect(
      await screen.findByText("Kann sicher schwimmen"),
    ).toBeInTheDocument();

    rerender(
      <StudentStammdatenTab
        student={makeStudent({ id: "2", first_name: "Mia" })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect(
      await screen.findByText("Zusatzangaben werden geladen..."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Kann sicher schwimmen")).not.toBeInTheDocument();
  });

  it("calls onSave with submitted form data", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "Updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0]![0]).toMatchObject({ first_name: "Updated" });
  });

  it("blocks saving when privacy consent failed to load", async () => {
    fetchStudentPrivacyConsentMock.mockRejectedValueOnce(
      new Error("server unavailable"),
    );
    const onSave = vi.fn().mockResolvedValue(undefined);

    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    await screen.findByText(/Datenschutzeinwilligung konnte nicht geladen/);
    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "Updated" },
    });

    expect(screen.getByRole("button", { name: /Speichern/ })).toBeDisabled();
    expect(onSave).not.toHaveBeenCalled();
  });

  // Regression test for the silent-data-loss bug. The global test setup
  // returns `tenant: null` from useTenantSafe, so useStudentPhotosEnabled
  // resolves to `false` here — exactly the production scenario the fix
  // targets (feature off, response strips photo_consent_given_at, but the
  // DB row may still carry the consent stamp).
  //
  // Before the fix: buildDraft set `photo_consent_given: Boolean(undefined)
  // = false` and the form's PUT submitted that, which backend
  // applyPhotoConsent treated as a true→false withdrawal — silently
  // nilling the original photo_consent_given_at/_by audit columns.
  //
  // After the fix: the field stays out of the draft entirely when the
  // feature is off, so the PUT omits it and applyPhotoConsent no-ops.
  it("omits photo_consent_given from PUT when photo feature is off", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={
          // Source row carries a consent stamp, but the response would
          // strip _at when the feature is off — simulate that by leaving
          // photo_consent_given_at undefined.
          makeStudent({ photo_consent_given_at: undefined })
        }
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "Updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(submitted.first_name).toBe("Updated");
    // The critical assertion: photo_consent_given must NOT be present.
    // `Object.keys` rather than `expect(...).toBeUndefined()` because the
    // serialized JSON behaviour matters — undefined-valued keys are still
    // dropped, but explicit `false` would be sent.
    expect(Object.keys(submitted)).not.toContain("photo_consent_given");
  });

  it("shows submit error from validator", () => {
    validateStudentFormMock.mockReturnValue({ first_name: "bad" });
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    // Rerender with dirty form to simulate trigger submit
    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "X" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    expect(screen.getByTestId("common-sections")).toHaveAttribute(
      "data-error-count",
      "1",
    );
  });

  it("displays the hardcoded submit error banner when onSave throws", async () => {
    // The component now owns the submit flow (orchestrates pending photo
    // upload + student PUT). On any failure it surfaces a generic German
    // banner — we don't echo backend error strings into the UI to avoid
    // leaking implementation details.
    const onSave = vi.fn().mockRejectedValue(new Error("Save failed"));

    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "X" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(
        screen.getByText(
          /Fehler beim Speichern\. Bitte versuchen Sie es erneut\./,
        ),
      ).toBeInTheDocument(),
    );
  });

  // The proxy commits the privacy consent before the student PUT, in a
  // separate backend transaction. When the student write then fails, the
  // generic banner would tell the user nothing was saved while their consent
  // change is already stored — and cancelling the retry leaves it there.
  it("names the saved consent when the student write failed on top of it", async () => {
    const onSave = vi.fn().mockRejectedValue(
      Object.assign(new Error("Save failed"), {
        body: JSON.stringify({
          error: "Save failed",
          details: { privacy_consent_saved: true },
        }),
      }),
    );

    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "X" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(
        screen.getByText(
          /Die Datenschutzeinstellungen wurden bereits gespeichert, die übrigen Änderungen nicht\./,
        ),
      ).toBeInTheDocument(),
    );
  });

  it("rebuilds draft when a *different* student is selected (id changes)", () => {
    const { rerender } = render(
      <StudentStammdatenTab
        student={makeStudent({ id: "1", first_name: "Old" })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect((screen.getByTestId("first-name") as HTMLInputElement).value).toBe(
      "Old",
    );

    // Different student selected — id changes, draft must reset to the
    // new server-side state.
    rerender(
      <StudentStammdatenTab
        student={makeStudent({ id: "2", first_name: "New" })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect((screen.getByTestId("first-name") as HTMLInputElement).value).toBe(
      "New",
    );
  });

  it("does NOT wipe the draft on background refetches of the same student", () => {
    // Regression: a successful photo upload triggered a parent SWR mutate,
    // shipped a new `student` object reference with the same id, and the
    // form's other unsaved edits (a half-typed name, etc.) got silently
    // overwritten — leaving the user with a disabled Speichern button.
    const { rerender } = render(
      <StudentStammdatenTab
        student={makeStudent({ id: "1", first_name: "Anna" })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    const input = screen.getByTestId("first-name") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "Annabelle" } });
    expect(input.value).toBe("Annabelle");

    // Same id, fresh server snapshot (e.g. photo_url was just added). The
    // form must NOT discard the user's unsaved name edit.
    rerender(
      <StudentStammdatenTab
        student={makeStudent({
          id: "1",
          first_name: "Anna",
          photo_url: "/api/students/1/photo/p_abc.jpg",
        })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect(input.value).toBe("Annabelle");
  });

  it("updates the departure plan", () => {
    render(
      <StudentStammdatenTab
        student={makeStudent({ bus: false, bus_days: {} })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect(screen.getByTestId("departure-mon")).toHaveTextContent("alone");
    fireEvent.click(screen.getByTestId("departure-set-mon-bus"));
    expect(screen.getByTestId("departure-mon")).toHaveTextContent("bus");
  });

  it("clears field-level error when the field changes", () => {
    validateStudentFormMock.mockReturnValue({ first_name: "required" });
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "X" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));
    expect(screen.getByTestId("common-sections")).toHaveAttribute(
      "data-error-count",
      "1",
    );

    // Typing again clears the first_name error
    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "XY" },
    });
    expect(screen.getByTestId("common-sections")).toHaveAttribute(
      "data-error-count",
      "0",
    );
  });
});

// Laufgemeinschaft suite. A companion-plan 409 is a question, not a failure:
// the backend refuses and names the children and weekdays it would have to
// widen. The retry has to repeat those names — the backend re-checks against
// freshly locked rows and only widens what the confirmation actually covers,
// so a retry that confirms nothing earns the identical 409 again. These tests
// pin down where that list is read from.
describe("StudentStammdatenTab — companion plan conflicts", () => {
  const conflictPayload = {
    error: "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
    message:
      "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
    conflicts: [{ student_id: 42, weekdays: ["mon", "tue"] }],
  };

  function conflictError(
    extra: { body?: string; message?: string } = {},
  ): Error & { status?: number; body?: string } {
    const err = new Error(
      extra.message ??
        "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
    ) as Error & { status?: number; body?: string };
    err.status = 409;
    if (extra.body !== undefined) err.body = extra.body;
    return err;
  }

  beforeEach(() => {
    vi.clearAllMocks();
    fetchStudentPrivacyConsentMock.mockResolvedValue(null);
    fetchStudentEnrollmentExtraFieldsMock.mockResolvedValue([]);
    validateStudentFormMock.mockReturnValue({});
    // The stored links must be known before a conflict can be answered — the
    // tab only submits (and only retries) a companion list it actually read.
    fetchStudentCompanionsMock.mockResolvedValue([
      { companion_student_id: "42", weekdays: ["mon"] },
    ]);
    handleStudentFormSubmitMock.mockImplementation(
      async (
        event: Event & { preventDefault: () => void },
        _formData: Partial<Student>,
        validate: () => boolean,
        save: (data: Partial<Student>) => Promise<void>,
        setSaving: (v: boolean) => void,
        setErrors: (e: Record<string, string>) => void,
      ) => {
        event.preventDefault();
        if (!validate()) return;
        setSaving(true);
        try {
          await save(_formData);
        } catch (err) {
          setErrors({ submit: err instanceof Error ? err.message : "err" });
        } finally {
          setSaving(false);
        }
      },
    );
  });

  async function saveAndAnswerConflict(
    err: Error,
  ): Promise<ReturnType<typeof vi.fn>> {
    const onSave = vi
      .fn()
      .mockRejectedValueOnce(err)
      .mockResolvedValue(undefined);

    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    await waitFor(() => expect(fetchStudentCompanionsMock).toHaveBeenCalled());

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "Maja" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    const confirmButton = await screen.findByRole("button", {
      name: "Ergänzen und speichern",
    });
    fireEvent.click(confirmButton);

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
    return onSave;
  }

  it("confirms the children the 409 body named on the retry", async () => {
    // The CRUD service hands over the untouched response body; the German
    // sentence in `message` never carries the list, so reading the body is the
    // only way the retry can name Kind 42 for Montag and Dienstag.
    const onSave = await saveAndAnswerConflict(
      conflictError({ body: JSON.stringify(conflictPayload) }),
    );

    expect(onSave.mock.calls[1]![0]).toMatchObject({
      extend_companion_plans: true,
      confirmed_companion_extensions: [
        { companion_student_id: "42", weekdays: ["mon", "tue"] },
      ],
    });
  });

  it("falls back to the JSON embedded in the message when no body traveled", async () => {
    // Errors from studentService reach this form without a body. The older
    // regex path stays alive so those callers keep working.
    const onSave = await saveAndAnswerConflict(
      conflictError({
        message: `API error (409): ${JSON.stringify(conflictPayload)}`,
      }),
    );

    expect(onSave.mock.calls[1]![0]).toMatchObject({
      confirmed_companion_extensions: [
        { companion_student_id: "42", weekdays: ["mon", "tue"] },
      ],
    });
  });

  it("still reads the message when the body carries no conflict list", async () => {
    // A body that is only the plain German sentence must not shadow a message
    // that does hold the list — otherwise the retry silently confirms nothing.
    const onSave = await saveAndAnswerConflict(
      conflictError({
        body: "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
        message: `API error (409): ${JSON.stringify(conflictPayload)}`,
      }),
    );

    expect(onSave.mock.calls[1]![0]).toMatchObject({
      confirmed_companion_extensions: [
        { companion_student_id: "42", weekdays: ["mon", "tue"] },
      ],
    });
  });

  it("drops the conflicts when the question is dismissed", async () => {
    // "Abbrechen" is a NO. Keeping the named children around would widen the
    // linked child's Heimweg on the next ordinary save — a cross-child write
    // the user declined.
    const onSave = vi
      .fn()
      .mockRejectedValueOnce(
        conflictError({ body: JSON.stringify(conflictPayload) }),
      )
      .mockResolvedValue(undefined);

    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );
    await waitFor(() => expect(fetchStudentCompanionsMock).toHaveBeenCalled());

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "Maja" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    fireEvent.click(await screen.findByRole("button", { name: "Abbrechen" }));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
    expect(onSave.mock.calls[1]![0]).toMatchObject({
      confirmed_companion_extensions: [],
    });
  });

  it("confirms nothing when neither body nor message can be read", async () => {
    // The safe direction: an unreadable conflict means the backend asks again
    // rather than the form widening a child's plan nobody agreed to.
    const onSave = await saveAndAnswerConflict(conflictError());

    expect(onSave.mock.calls[1]![0]).toMatchObject({
      extend_companion_plans: true,
      confirmed_companion_extensions: [],
    });
  });
});

// Staleness suite. The submitted companion list REPLACES the stored one, so it
// may only travel when this form actually edited it: someone else may have
// changed the links since the load, and re-sending the loaded snapshot on an
// unrelated save would silently revert their change (the backend row-locks the
// writes, but a lock cannot tell that ours is stale).
describe("StudentStammdatenTab — companion list staleness", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchStudentPrivacyConsentMock.mockResolvedValue(null);
    fetchStudentEnrollmentExtraFieldsMock.mockResolvedValue([]);
    validateStudentFormMock.mockReturnValue({});
    fetchStudentCompanionsMock.mockResolvedValue([
      { companion_student_id: "42", weekdays: ["mon"] },
    ]);
    handleStudentFormSubmitMock.mockImplementation(
      async (
        event: Event & { preventDefault: () => void },
        _formData: Partial<Student>,
        validate: () => boolean,
        save: (data: Partial<Student>) => Promise<void>,
        setSaving: (v: boolean) => void,
      ) => {
        event.preventDefault();
        if (!validate()) return;
        setSaving(true);
        await save(_formData);
        setSaving(false);
      },
    );
  });

  async function saveAfter(
    edit: () => void,
  ): Promise<ReturnType<typeof vi.fn>> {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );
    await waitFor(() => expect(fetchStudentCompanionsMock).toHaveBeenCalled());
    edit();
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    return onSave;
  }

  it("omits the companion list from a save that did not touch it", async () => {
    const onSave = await saveAfter(() =>
      fireEvent.change(screen.getByTestId("address-street"), {
        target: { value: "Neue Straße 1" },
      }),
    );

    // No key at all — the backend leaves the stored links alone. An empty list
    // would delete them, the loaded list would overwrite a concurrent edit.
    expect(onSave.mock.calls[0]![0]).not.toHaveProperty("companions");
  });

  it("sends the companion list once the user edited it", async () => {
    const onSave = await saveAfter(() =>
      fireEvent.click(screen.getByTestId("companion-add")),
    );

    expect(onSave.mock.calls[0]![0]).toMatchObject({
      companions: [{ companion_student_id: "7", weekdays: ["mon"] }],
    });
  });

  // The list REPLACES the stored one, so the backend has to be told which
  // stored list it replaces. Without it two people editing the same child from
  // the same snapshot both submit everything they saw and the second save
  // silently deletes the first one's link.
  it("sends the fingerprint of the loaded list along with an edited one", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([
      { companion_student_id: "2", weekdays: ["mon"] },
    ]);
    const onSave = await saveAfter(() =>
      fireEvent.click(screen.getByTestId("companion-add")),
    );

    expect(onSave.mock.calls[0]![0]).toMatchObject({
      companions_fingerprint: "2:mon",
    });
  });

  // An unrelated save must not claim anything about the links — it does not
  // replace them, so a fingerprint could only ever refuse it for someone else's
  // legitimate change.
  it("omits the fingerprint from a save that does not touch the links", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([
      { companion_student_id: "2", weekdays: ["mon"] },
    ]);
    const onSave = await saveAfter(() =>
      fireEvent.change(screen.getByTestId("address-street"), {
        target: { value: "Neue Straße 1" },
      }),
    );

    expect(onSave.mock.calls[0]![0]).not.toHaveProperty(
      "companions_fingerprint",
    );
  });

  // The tab stays mounted while other people work on the same child, and
  // student.id does not change when they do — so without listening to
  // companion writes the form keeps a snapshot that no longer exists and its
  // next save replaces the stored links with it.
  it("reloads the links on a remote change while nothing is edited", async () => {
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn().mockResolvedValue(undefined)}
      />,
    );
    // The picker only appears once the stored links are known.
    expect(await screen.findByTestId("companion-add")).toBeInTheDocument();
    expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(1);

    act(() => {
      notifyStudentCompanionsChanged();
    });

    await waitFor(() =>
      expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(2),
    );
  });

  it("blocks the save when a remote change lands on an edited list", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );
    expect(await screen.findByTestId("companion-add")).toBeInTheDocument();
    expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTestId("companion-add"));
    act(() => {
      notifyStudentCompanionsChanged();
    });

    // The draft survives — discarding the user's work silently would be the
    // other way to lose an edit.
    expect(
      await screen.findByText(/zwischenzeitlich an anderer Stelle geändert/),
    ).toBeInTheDocument();
    expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));
    await waitFor(() =>
      expect(
        screen.getByText(/Bitte oben neu laden und die Änderung wiederholen/),
      ).toBeInTheDocument(),
    );
    expect(onSave).not.toHaveBeenCalled();

    // "Neu laden" is the explicit way out.
    fireEvent.click(screen.getByRole("button", { name: /Neu laden/ }));
    await waitFor(() =>
      expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(2),
    );
  });

  // The other side of the same coin: because a narrowed plan DELETES links
  // without naming one, that save has to say which links it was looking at. The
  // backend refuses a plan-driven removal that makes no such claim, so a form
  // whose plan went stale (a dropped refresh) is told to reload instead of
  // quietly dropping an edge somebody else just created.
  it("sends the fingerprint with a plan edit that carries no list", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([
      { companion_student_id: "2", weekdays: ["mon"] },
    ]);
    const onSave = await saveAfter(() =>
      fireEvent.click(screen.getByTestId("departure-set-mon-bus")),
    );

    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(submitted).not.toHaveProperty("companions");
    expect(submitted).toMatchObject({ companions_fingerprint: "2:mon" });
  });

  // A departure-plan save can change the stored links without carrying a
  // companions list: the backend TRIMS every link whose weekday the new plan no
  // longer allows, and the picker unmounts with the last accompanied day before
  // it can trim the local copy. Promoting the list this form still holds to the
  // new baseline would resurrect the deleted children here. student.id does not
  // change on a refetch, so only an explicit reload re-reads them.
  it("re-reads the stored links after a save instead of trusting the local list", async () => {
    await saveAfter(() =>
      fireEvent.click(screen.getByTestId("departure-set-mon-bus")),
    );

    await waitFor(() =>
      expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(2),
    );
  });

  // The stranded-companion 400 is an instruction, not a failure: it names the
  // child whose Heimweg has to be filled in first. "Bitte versuchen Sie es
  // erneut" would be wrong twice — the identical retry is refused again, and
  // the only way out of the refusal is lost.
  it("shows the backend message when a link would strand the other child", async () => {
    const refusal = new Error(
      "API error 400: Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht.",
    ) as Error & { status?: number; body?: string };
    refusal.status = 400;
    refusal.body = JSON.stringify({
      status: "error",
      error:
        "Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.",
      code: "companion_would_lose_departure",
    });

    const onSave = vi.fn().mockRejectedValue(refusal);
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );
    await waitFor(() => expect(fetchStudentCompanionsMock).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId("companion-add"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    expect(
      await screen.findByText(
        "Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.",
      ),
    ).toBeInTheDocument();
  });
});

// Photo orchestration suite. The default global useTenantSafe mock returns
// `tenant: null` so photosEnabled resolves to false — that's covered by the
// existing tests above. These tests override the mock to flip the feature on
// and exercise the submit-time photo upload/delete branches that were
// previously uncovered.
describe("StudentStammdatenTab — photo orchestration", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    fetchStudentPrivacyConsentMock.mockResolvedValue(null);
    validateStudentFormMock.mockReturnValue({});
    // Override useTenantSafe so photosEnabled is true for this suite.
    const tenantProvider = await import("~/lib/tenant-context");
    vi.mocked(tenantProvider.useTenantSafe).mockReturnValue({
      tenantSlug: "test-tenant",
      tenant: { studentPhotosEnabled: true },
    } as unknown as ReturnType<typeof tenantProvider.useTenantSafe>);
    uploadStudentPhotoMock.mockResolvedValue({
      photoUrl: "/api/students/1/photo/p_new.jpg",
    });
    deleteStudentPhotoMock.mockResolvedValue(undefined);
  });

  it("uploads the pending photo AFTER the student PUT succeeds", async () => {
    // The order matters — submitting photo before PUT can leave the server in
    // a half-saved state if the PUT fails (file already swapped, but other
    // form fields rejected). The component does PUT → photo upload, in that
    // order.
    const callOrder: string[] = [];
    const onSave = vi.fn().mockImplementation(async () => {
      callOrder.push("onSave");
    });
    uploadStudentPhotoMock.mockImplementation(async () => {
      callOrder.push("upload");
      return { photoUrl: "/api/students/1/photo/p_new.jpg" };
    });

    render(
      <StudentStammdatenTab
        student={makeStudent({
          photo_consent_given_at: "2026-01-01T00:00:00Z",
        })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByTestId("pick-photo"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(uploadStudentPhotoMock).toHaveBeenCalled());
    expect(callOrder).toEqual(["onSave", "upload"]);
    expect(uploadStudentPhotoMock).toHaveBeenCalledWith("1", expect.any(Blob), {
      consentAcknowledged: true,
    });
    // Existing consent was not changed in this form, so the PUT omits it.
    expect(
      Object.keys(onSave.mock.calls[0]![0] as Partial<Student>),
    ).not.toContain("photo_consent_given");
  });

  it("sends photo_consent_given only when the checkbox changed", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);

    render(
      <StudentStammdatenTab
        student={makeStudent({ photo_consent_given_at: undefined })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByTestId("consent-on"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0]![0]).toMatchObject({
      photo_consent_given: true,
    });
  });

  it("omits unchanged false consent so stale forms cannot withdraw new consent", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);

    render(
      <StudentStammdatenTab
        student={makeStudent({ photo_consent_given_at: undefined })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "Updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(submitted.first_name).toBe("Updated");
    expect(Object.keys(submitted)).not.toContain("photo_consent_given");
  });

  it("sends false when the user explicitly withdraws consent", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);

    render(
      <StudentStammdatenTab
        student={makeStudent({
          photo_consent_given_at: "2026-01-01T00:00:00Z",
        })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByTestId("consent-off"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0]![0]).toMatchObject({
      photo_consent_given: false,
    });
  });

  it("deletes the photo AFTER the student PUT when removal is pending", async () => {
    const callOrder: string[] = [];
    const onSave = vi.fn().mockImplementation(async () => {
      callOrder.push("onSave");
    });
    deleteStudentPhotoMock.mockImplementation(async () => {
      callOrder.push("delete");
    });

    render(
      <StudentStammdatenTab
        student={makeStudent({
          photo_consent_given_at: "2026-01-01T00:00:00Z",
          photo_url: "/api/students/1/photo/p_old.jpg",
        })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByTestId("mark-removed"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(deleteStudentPhotoMock).toHaveBeenCalled());
    expect(callOrder).toEqual(["onSave", "delete"]);
    expect(deleteStudentPhotoMock).toHaveBeenCalledWith("1");
    // PUT must NOT have triggered uploadStudentPhoto.
    expect(uploadStudentPhotoMock).not.toHaveBeenCalled();
  });

  it("surfaces a partial-success error when the photo upload fails after a successful PUT", async () => {
    // The component's contract: data fields ARE saved (PUT succeeded) but the
    // photo step failed. The user sees a partial-success banner and the
    // pending photo state is preserved so a re-click of Speichern replays
    // just the photo mutation.
    const onSave = vi.fn().mockResolvedValue(undefined);
    uploadStudentPhotoMock.mockRejectedValue(new Error("network down"));

    render(
      <StudentStammdatenTab
        student={makeStudent({
          photo_consent_given_at: "2026-01-01T00:00:00Z",
        })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByTestId("pick-photo"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(
        screen.getByText(
          /Daten gespeichert, aber das Foto konnte nicht aktualisiert werden\. Bitte versuchen Sie es erneut\./,
        ),
      ).toBeInTheDocument(),
    );
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(uploadStudentPhotoMock).toHaveBeenCalledTimes(1);
  });

  it("calls onStudentRefresh after a successful photo upload", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    const onStudentRefresh = vi.fn().mockResolvedValue(undefined);

    render(
      <StudentStammdatenTab
        student={makeStudent({
          photo_consent_given_at: "2026-01-01T00:00:00Z",
        })}
        groups={[]}
        onSave={onSave}
        onStudentRefresh={onStudentRefresh}
      />,
    );

    fireEvent.click(screen.getByTestId("pick-photo"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onStudentRefresh).toHaveBeenCalled());
    expect(uploadStudentPhotoMock).toHaveBeenCalledTimes(1);
  });

  it("does not crash when onStudentRefresh throws — logs only", async () => {
    // Refresh failures must NOT surface as save errors; the photo mutation
    // already committed and the next list-level refetch will reconcile.
    const onSave = vi.fn().mockResolvedValue(undefined);
    const onStudentRefresh = vi
      .fn()
      .mockRejectedValue(new Error("refresh boom"));

    render(
      <StudentStammdatenTab
        student={makeStudent({
          photo_consent_given_at: "2026-01-01T00:00:00Z",
        })}
        groups={[]}
        onSave={onSave}
        onStudentRefresh={onStudentRefresh}
      />,
    );

    fireEvent.click(screen.getByTestId("pick-photo"));
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onStudentRefresh).toHaveBeenCalled());
    // No partial-success banner should appear — the photo mutation succeeded.
    expect(
      screen.queryByText(
        /Daten gespeichert, aber das Foto konnte nicht aktualisiert werden\. Bitte versuchen Sie es erneut\./,
      ),
    ).not.toBeInTheDocument();
  });

  it("syncs photo_consent_given when the feature flips on for the same student", async () => {
    // Off → on: the dedicated effect pulls the server's consent state into
    // the draft so the checkbox renders the right initial value. This test
    // first renders with the feature OFF (consent omitted from draft) then
    // flips it ON (server has consent recorded → draft picks up `true`).
    const tenantProvider = await import("~/lib/tenant-context");
    // Start with feature off.
    vi.mocked(tenantProvider.useTenantSafe).mockReturnValue({
      tenantSlug: "test-tenant",
      tenant: null,
    } as unknown as ReturnType<typeof tenantProvider.useTenantSafe>);

    const onSave = vi.fn().mockResolvedValue(undefined);
    const student = makeStudent({
      photo_consent_given_at: "2026-01-01T00:00:00Z",
    });
    const { rerender } = render(
      <StudentStammdatenTab student={student} groups={[]} onSave={onSave} />,
    );

    // Photo section is hidden while feature is off.
    expect(screen.queryByTestId("photo-section")).toBeNull();

    // Flip feature on — re-render with the same student.
    vi.mocked(tenantProvider.useTenantSafe).mockReturnValue({
      tenantSlug: "test-tenant",
      tenant: { studentPhotosEnabled: true },
    } as unknown as ReturnType<typeof tenantProvider.useTenantSafe>);
    rerender(
      <StudentStammdatenTab student={student} groups={[]} onSave={onSave} />,
    );

    // Photo section now visible. consentGiven derived from the synced field.
    expect(screen.getByTestId("photo-section")).toHaveAttribute(
      "data-consent",
      "true",
    );

    // Flip feature back off — section disappears, consent key dropped from
    // submitted draft. Trigger a save to assert the field is no longer in
    // the PUT body.
    vi.mocked(tenantProvider.useTenantSafe).mockReturnValue({
      tenantSlug: "test-tenant",
      tenant: null,
    } as unknown as ReturnType<typeof tenantProvider.useTenantSafe>);
    rerender(
      <StudentStammdatenTab student={student} groups={[]} onSave={onSave} />,
    );
    expect(screen.queryByTestId("photo-section")).toBeNull();

    // Make the form dirty (so Speichern enables) and submit.
    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "Updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));
    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(Object.keys(submitted)).not.toContain("photo_consent_given");
  });

  it("rebases an untouched departure plan onto the refreshed student", async () => {
    // Somebody else widened the plan (and linked a child on the new day) while
    // this form sat open on an address edit. The form resubmits the plan on
    // every save and the backend trims the links to it, so a stale plan here
    // would delete that fresh link.
    const onSave = vi.fn().mockResolvedValue(undefined);
    const { rerender } = render(
      <StudentStammdatenTab
        student={makeStudent({
          allowed_departure_modes: { mon: ["accompanied"] },
        })}
        groups={[]}
        onSave={onSave}
      />,
    );

    rerender(
      <StudentStammdatenTab
        student={makeStudent({
          allowed_departure_modes: {
            mon: ["accompanied"],
            tue: ["accompanied"],
          },
        })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("address-city"), {
      target: { value: "Bonn" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(submitted.allowed_departure_modes).toEqual({
      mon: ["accompanied"],
      tue: ["accompanied"],
    });
  });

  it("keeps a locally edited departure plan when the student refreshes", async () => {
    // The other half of the rule: a plan the user actually changed is their
    // edit and must survive a background refetch — the rebase may only move a
    // plan nobody touched.
    const onSave = vi.fn().mockResolvedValue(undefined);
    const { rerender } = render(
      <StudentStammdatenTab
        student={makeStudent({
          allowed_departure_modes: { mon: ["accompanied"] },
        })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByTestId("departure-set-mon-bus"));

    rerender(
      <StudentStammdatenTab
        student={makeStudent({
          allowed_departure_modes: {
            mon: ["accompanied"],
            tue: ["accompanied"],
          },
        })}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const submitted = onSave.mock.calls[0]![0] as Partial<Student>;
    expect(submitted.allowed_departure_modes).toEqual({ mon: ["bus"] });
  });
});
