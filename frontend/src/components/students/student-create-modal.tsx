"use client";

import { useState, useEffect, useRef } from "react";
import { Plus, Search, Trash2 } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Modal } from "~/components/ui/modal";
import { Input } from "~/components/ui/input";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useScrollToFirstError } from "~/lib/hooks/use-scroll-to-error";
import type { Student } from "@/lib/api";
import { PersonalInfoSection, DepartureSection } from "./student-form-fields";
import { StudentCommonFormSections } from "./student-common-form-sections";
import {
  validateStudentForm,
  handleStudentFormSubmit,
} from "~/lib/student-form-validation";
import {
  busDaysHaveAny,
  pickupDaysHaveAny,
  type BusDays,
  allowedDepartureToBusDays,
  allowedDepartureToDepartureDays,
  allowedDepartureToPickupDays,
  normalizeAllowedDepartureModes,
} from "~/lib/student-helpers";
import GuardianFormModal, {
  type RelationshipFormData,
} from "~/components/guardians/guardian-form-modal";
import GuardianPickerPanel from "~/components/guardians/guardian-picker-panel";
import {
  RELATIONSHIP_TYPES,
  type Guardian,
  type GuardianFormData,
  type PhoneType,
  type StudentGuardianPayload,
} from "@/lib/guardian-helpers";
import { CareWeeklyPlanModal } from "./care-weekly-plan-modal";
import {
  WEEKDAYS,
  type ArrivalScheduleFormEntry,
} from "~/lib/arrival-schedule-helpers";
import {
  fetchArrivalSettings,
  type CareDaysSource,
} from "~/lib/student-arrival-api";
import {
  mapBulkPickupScheduleFormToBackend,
  type BackendPickupScheduleRequest,
  type PickupScheduleFormData,
} from "~/lib/pickup-schedule-helpers";

// Shape returned by GuardianFormModal's onSubmit for each entry.
type GuardianSubmitEntry = {
  id: string;
  guardianData: GuardianFormData;
  relationshipData: RelationshipFormData;
  phoneNumbers: Array<{
    id?: string;
    phoneNumber: string;
    phoneType: PhoneType;
    label?: string;
    isPrimary: boolean;
  }>;
};

// Maps GuardianFormModal entries onto the snake_case payload the create-student
// endpoint expects, so the same component used on the detail page feeds the
// atomic create flow without rebuilding any guardian fields.
function toGuardianPayloads(
  entries: GuardianSubmitEntry[],
): StudentGuardianPayload[] {
  return entries.map((entry) => ({
    first_name: entry.guardianData.firstName,
    last_name: entry.guardianData.lastName,
    email: entry.guardianData.email,
    address_street: entry.guardianData.addressStreet,
    address_city: entry.guardianData.addressCity,
    address_postal_code: entry.guardianData.addressPostalCode,
    // preferred_contact_method and pickup_notes are part of the shared guardian
    // types but GuardianFormModal's create mode does not expose inputs for them
    // yet, so they arrive undefined today. Mapped through deliberately so they
    // flow automatically once the form adds those fields (the backend applies
    // its own contact-method default when omitted).
    preferred_contact_method: entry.guardianData.preferredContactMethod,
    language_preference: entry.guardianData.languagePreference,
    notes: entry.guardianData.notes,
    relationship_type: entry.relationshipData.relationshipType,
    guardian_role: entry.relationshipData.guardianRole,
    is_primary: entry.relationshipData.isPrimary,
    is_emergency_contact: entry.relationshipData.isEmergencyContact,
    can_pickup: entry.relationshipData.canPickup,
    pickup_notes: entry.relationshipData.pickupNotes,
    emergency_priority: entry.relationshipData.emergencyPriority,
    phone_numbers: entry.phoneNumbers.map((phone) => ({
      phone_number: phone.phoneNumber,
      phone_type: phone.phoneType,
      label: phone.label,
      is_primary: phone.isPrimary,
    })),
  }));
}

// Maps a guardian chosen from the picker (an EXISTING profile) plus the
// relationship flags onto the create-student payload. guardian_profile_id tells
// the backend to link the existing profile instead of creating a new one; the
// name rides along only for the summary list and is ignored server-side. The
// email is deliberately NOT carried — the backend ignores profile fields for an
// existing link anyway, so there's no reason to round-trip it.
function toExistingGuardianPayload(
  guardian: Guardian,
  relationship: RelationshipFormData,
): StudentGuardianPayload {
  return {
    guardian_profile_id: Number(guardian.id),
    first_name: guardian.firstName,
    last_name: guardian.lastName,
    relationship_type: relationship.relationshipType,
    guardian_role: relationship.guardianRole,
    is_primary: relationship.isPrimary,
    is_emergency_contact: relationship.isEmergencyContact,
    can_pickup: relationship.canPickup,
    emergency_priority: relationship.emergencyPriority,
    phone_numbers: [],
  };
}

// Human-readable relationship label for the summary list.
function relationshipLabel(value: string): string {
  return RELATIONSHIP_TYPES.find((t) => t.value === value)?.label ?? value;
}

// Weekly schedules travel to the backend in snake_case alongside the student so
// they are persisted in the same atomic create transaction. Arrival entries are
// already backend-shaped (ArrivalScheduleFormEntry); pickup entries are mapped
// from the care-plan form via mapBulkPickupScheduleFormToBackend.
export type CreateStudentSchedules = {
  arrival_schedules?: ArrivalScheduleFormEntry[];
  pickup_schedules?: BackendPickupScheduleRequest[];
};

// The two things "+ Kinder" can create (#2382): a regular OGS child, or a
// minimal class-list-only entry (Name + Klasse, "Keine Betreuung").
type CreateMode = "student" | "list-entry";

interface StudentCreateModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (
    data: Partial<Student> & {
      guardians?: StudentGuardianPayload[];
    } & CreateStudentSchedules,
  ) => Promise<void>;
  /**
   * Creates a class-list-only entry (#2382). When set, the modal offers the
   * "Nur Klassenliste" mode switch — the admin's mental model is one entry
   * point for "Kind erfassen", so the choice lives HERE, not on a page the
   * user must know about beforehand. Optional: callers without the feature
   * get the unchanged student form.
   */
  readonly onCreateListEntry?: (input: {
    firstName: string;
    lastName: string;
    schoolClass: string;
  }) => Promise<void>;
  readonly groups?: Array<{ readonly value: string; readonly label: string }>;
}

const EMPTY_GROUPS: NonNullable<StudentCreateModalProps["groups"]> = [];

export function StudentCreateModal({
  isOpen,
  onClose,
  onCreate,
  onCreateListEntry,
  groups = EMPTY_GROUPS,
}: StudentCreateModalProps) {
  const [mode, setMode] = useState<CreateMode>("student");
  const [formData, setFormData] = useState<Partial<Student>>({
    first_name: "",
    second_name: "",
    school_class: "",
    group_id: "",
    birthday: "",
    health_info: "",
    supervisor_notes: "",
    extra_info: "",
    privacy_consent_accepted: false,
    data_retention_days: 30,
    bus: false,
    // Explicit empty pickup map = "Geht alleine nach Hause". Sending it (not
    // omitting it) keeps the create contract identical to the edit flow and
    // makes the stored pickup_days/pickup_status pair correct even when the
    // pickup section is never touched.
    pickup_days: {},
    departure_days: {},
    allowed_departure_modes: {},
    pickup_status: "Geht alleine nach Hause",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saveLoading, setSaveLoading] = useState(false);
  const [guardians, setGuardians] = useState<StudentGuardianPayload[]>([]);
  const [guardianModalOpen, setGuardianModalOpen] = useState(false);
  // Weekly care plan staged in the create modal (no API calls here — persisted
  // atomically with the student on submit). Arrival entries are backend-shaped;
  // pickup entries stay in the care-plan form shape until submit.
  const [arrivalSchedules, setArrivalSchedules] = useState<
    ArrivalScheduleFormEntry[]
  >([]);
  const [pickupSchedules, setPickupSchedules] = useState<
    PickupScheduleFormData[]
  >([]);
  const [carePlanModalOpen, setCarePlanModalOpen] = useState(false);
  const [careDaysSource, setCareDaysSource] = useState<CareDaysSource | null>(
    null,
  );
  const [arrivalSettingsLoading, setArrivalSettingsLoading] = useState(false);
  const [arrivalSettingsLoadError, setArrivalSettingsLoadError] =
    useState(false);
  const [guardianPickerOpen, setGuardianPickerOpen] = useState(false);
  // The inline picker panel is tall; collapsing it (on add or cancel) shrinks
  // the modal so the kept scroll position lands further down the form. Re-anchor
  // to the guardian section so the user stays where they acted and sees the
  // result (the added guardian) instead of jumping past it.
  const guardianSectionRef = useRef<HTMLElement>(null);
  const pendingGuardianScrollRef = useRef(false);
  // Scroll the form to the first invalid field on a failed submit — the modal
  // body scrolls and the submit button sits at the bottom, so an error above
  // is otherwise easy to miss. Same behaviour as the parents' enrollment form.
  const { formRef, errorRef, scrollToError } = useScrollToFirstError();

  // Reset form when modal opens/closes
  useEffect(() => {
    if (isOpen) {
      setMode("student");
      setFormData({
        first_name: "",
        second_name: "",
        school_class: "",
        group_id: "",
        birthday: "",
        health_info: "",
        supervisor_notes: "",
        extra_info: "",
        privacy_consent_accepted: false,
        data_retention_days: 30,
        bus: false,
        pickup_days: {},
        departure_days: {},
        pickup_status: "Geht alleine nach Hause",
      });
      setErrors({});
      setGuardians([]);
      setGuardianModalOpen(false);
      setArrivalSchedules([]);
      setPickupSchedules([]);
      setCarePlanModalOpen(false);
      setCareDaysSource(null);
      setArrivalSettingsLoading(false);
      setArrivalSettingsLoadError(false);
      setGuardianPickerOpen(false);
      pendingGuardianScrollRef.current = false;
    }
  }, [isOpen]);

  // The tenant decides where care days come from. Resolve that boundary as
  // soon as the dialog opens: in booking mode this direct form must not create
  // an OGS child without the approved offering links that define its days.
  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    setArrivalSettingsLoading(true);
    setArrivalSettingsLoadError(false);
    void fetchArrivalSettings()
      .then((settings) => {
        if (!cancelled) setCareDaysSource(settings.care_days_source);
      })
      .catch(() => {
        if (!cancelled) {
          setCareDaysSource(null);
          setArrivalSettingsLoadError(true);
        }
      })
      .finally(() => {
        if (!cancelled) setArrivalSettingsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen]);

  // After the inline picker collapses (add or cancel), re-anchor to the guardian
  // section so the shrinking modal doesn't leave the user scrolled past it.
  useEffect(() => {
    if (pendingGuardianScrollRef.current) {
      pendingGuardianScrollRef.current = false;
      guardianSectionRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "nearest",
      });
    }
  }, [guardians, guardianPickerOpen]);

  // Collect guardians from the reused GuardianFormModal into local state. The
  // guardians are persisted together with the student in one request — no API
  // calls happen here.
  const handleGuardianSubmit = async (entries: GuardianSubmitEntry[]) => {
    setGuardians((prev) => [...prev, ...toGuardianPayloads(entries)]);
    setGuardianModalOpen(false);
  };

  // Link an existing guardian chosen from the picker. Dedup defensively: if the
  // same profile is already in the list, skip it (the picker also greys out
  // already-added profiles, so this only guards a race).
  const handleGuardianPick = (
    guardian: Guardian,
    relationship: RelationshipFormData,
  ) => {
    const profileId = Number(guardian.id);
    setGuardians((prev) =>
      prev.some((g) => g.guardian_profile_id === profileId)
        ? prev
        : [...prev, toExistingGuardianPayload(guardian, relationship)],
    );
    pendingGuardianScrollRef.current = true;
    setGuardianPickerOpen(false);
  };

  // Collapse the inline picker without selecting; re-anchor like the add path.
  const handlePickerCancel = () => {
    pendingGuardianScrollRef.current = true;
    setGuardianPickerOpen(false);
  };

  const removeGuardian = (index: number) => {
    setGuardians((prev) => prev.filter((_, i) => i !== index));
  };

  const handleOpenCarePlan = () => {
    if (careDaysSource === "weekly_plan") {
      setCarePlanModalOpen(true);
    }
  };

  // Profile ids already added — passed to the picker so it greys out
  // guardians that are already on this child.
  const addedProfileIds = guardians
    .map((g) => g.guardian_profile_id)
    .filter((id): id is number => id !== undefined)
    .map((id) => id.toString());

  const validateForm = (): boolean => {
    const newErrors = validateStudentForm(formData, {
      firstName: true,
      lastName: true,
      schoolClass: true,
    });
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    // Class-list-only entry (#2382): three fields, one call — the same
    // validation flags the student path uses (Vorname/Nachname/Klasse).
    if (mode === "list-entry" && onCreateListEntry) {
      e.preventDefault();
      if (!validateForm()) {
        scrollToError();
        return;
      }
      setSaveLoading(true);
      return onCreateListEntry({
        firstName: (formData.first_name ?? "").trim(),
        lastName: (formData.second_name ?? "").trim(),
        schoolClass: (formData.school_class ?? "").trim(),
      })
        .then(() => onClose())
        .catch((error: unknown) => {
          setErrors({
            submit:
              error instanceof Error
                ? error.message
                : "Eintrag konnte nicht angelegt werden",
          });
        })
        .finally(() => setSaveLoading(false));
    }
    if (careDaysSource !== "weekly_plan") {
      e.preventDefault();
      return;
    }
    const payload: Partial<Student> & {
      guardians?: StudentGuardianPayload[];
    } & CreateStudentSchedules = { ...formData };
    if (guardians.length > 0) {
      payload.guardians = guardians;
    }
    if (arrivalSchedules.length > 0) {
      payload.arrival_schedules = arrivalSchedules;
    }
    if (pickupSchedules.length > 0) {
      payload.pickup_schedules = mapBulkPickupScheduleFormToBackend({
        schedules: pickupSchedules,
      }).schedules;
    }
    return handleStudentFormSubmit(
      e,
      payload,
      validateForm,
      onCreate,
      setSaveLoading,
      setErrors,
      scrollToError,
    );
  };

  const handleChange = (
    field: keyof Student,
    value: string | boolean | number | BusDays | null,
  ) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    // Clear error for this field
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  // Weekdays covered by either an arrival or a pickup time, for the summary.
  const scheduledWeekdays = Array.from(
    new Set([
      ...arrivalSchedules.map((s) => s.weekday),
      ...pickupSchedules.map((s) => s.weekday),
    ]),
  ).sort((a, b) => a - b);
  const hasCarePlan = scheduledWeekdays.length > 0;
  const studentFormAvailable =
    mode === "student" && careDaysSource === "weekly_plan";
  const weekdayShort = (value: number) =>
    WEEKDAYS.find((d) => d.value === value)?.shortLabel ?? String(value);

  return (
    <>
      <Modal isOpen={isOpen} onClose={onClose} title="Neues Kind">
        <form
          ref={formRef}
          onSubmit={handleSubmit}
          noValidate
          className="space-y-4 md:space-y-6"
        >
          {/* Submit Error */}
          {errors.submit && (
            <div
              ref={errorRef}
              className="rounded-lg border border-red-200 bg-red-50 p-2 md:p-3"
            >
              <p className="text-xs text-red-800 md:text-sm">{errors.submit}</p>
            </div>
          )}

          {/* Art des Eintrags (#2382): reguläres OGS-Kind oder minimaler
              Klassenlisteneintrag. Die Weiche sitzt im Erstell-Modal, weil
              "Kind erfassen" der eine Einstieg ist, den alle kennen. */}
          {onCreateListEntry ? (
            <SegmentedControl
              ariaLabel="Art des Eintrags"
              fullWidth
              items={[
                { value: "student", label: "Mit OGS-Betreuung" },
                { value: "list-entry", label: "Nur Klassenliste" },
              ]}
              value={mode}
              onChange={(next) => {
                setMode(next);
                setErrors({});
              }}
            />
          ) : null}

          {mode === "student" && arrivalSettingsLoading ? (
            <Alert
              type="info"
              message="Die Einstellungen für Betreuungstage werden geladen."
            />
          ) : null}

          {mode === "student" && arrivalSettingsLoadError ? (
            <Alert
              type="error"
              message="Die Betreuungstage konnten nicht geladen werden. Schließen Sie das Fenster und öffnen Sie es erneut."
            />
          ) : null}

          {mode === "student" && careDaysSource === "bookings" ? (
            <Alert
              type="info"
              message="Die Betreuungstage dieser Schule kommen aus Buchungen. Legen Sie ein Kind mit OGS-Betreuung deshalb unter Anmeldephasen mit „Manuelle Anmeldung“ an."
              action={
                <ButtonLink
                  href="/enrollment-phases"
                  variant="surface"
                  size="compact"
                >
                  Anmeldephasen öffnen
                </ButtonLink>
              }
            />
          ) : null}

          {mode === "list-entry" ? (
            <div className="space-y-4">
              <div className="border-moto-blue/20 bg-moto-blue-soft rounded-xl border p-3 text-xs leading-5 text-gray-700 md:p-4 md:text-sm">
                Für Kinder, die nicht im Ganztag sind: Der Eintrag besteht nur
                aus Name und Klasse und erscheint ausschließlich auf
                Klassenlisten und in der Klassenansicht (
                <span className="font-medium">Keine Betreuung</span>). Keine
                Anwesenheit, keine Betreuungsplanung, keine Kontaktdaten.
                Verwalten lässt sich die Liste über das Menü oben rechts unter{" "}
                <span className="font-medium">Klassenliste</span>.
              </div>
              <div className="rounded-xl border border-gray-100 bg-gray-50 p-3 md:p-4">
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
                  <div>
                    <label
                      htmlFor="list-entry-first-name"
                      className="mb-1 block text-xs font-medium text-gray-700 md:text-sm"
                    >
                      Vorname <span className="text-moto-red">*</span>
                    </label>
                    <Input
                      id="list-entry-first-name"
                      value={formData.first_name ?? ""}
                      onChange={(e) =>
                        handleChange("first_name", e.target.value)
                      }
                      placeholder="Max"
                    />
                    {errors.first_name ? (
                      <p className="text-moto-red mt-1 text-xs">
                        {errors.first_name}
                      </p>
                    ) : null}
                  </div>
                  <div>
                    <label
                      htmlFor="list-entry-last-name"
                      className="mb-1 block text-xs font-medium text-gray-700 md:text-sm"
                    >
                      Nachname <span className="text-moto-red">*</span>
                    </label>
                    <Input
                      id="list-entry-last-name"
                      value={formData.second_name ?? ""}
                      onChange={(e) =>
                        handleChange("second_name", e.target.value)
                      }
                      placeholder="Mustermann"
                    />
                    {errors.second_name ? (
                      <p className="text-moto-red mt-1 text-xs">
                        {errors.second_name}
                      </p>
                    ) : null}
                  </div>
                  <div>
                    <label
                      htmlFor="list-entry-school-class"
                      className="mb-1 block text-xs font-medium text-gray-700 md:text-sm"
                    >
                      Klasse <span className="text-moto-red">*</span>
                    </label>
                    <Input
                      id="list-entry-school-class"
                      value={formData.school_class ?? ""}
                      onChange={(e) =>
                        handleChange("school_class", e.target.value)
                      }
                      placeholder="5A"
                    />
                    {errors.school_class ? (
                      <p className="text-moto-red mt-1 text-xs">
                        {errors.school_class}
                      </p>
                    ) : (
                      <p className="mt-1 text-xs text-gray-500">
                        Genau wie bei den regulären Kindern geschrieben.
                      </p>
                    )}
                  </div>
                </div>
              </div>
            </div>
          ) : null}

          {/* Personal Information */}
          {studentFormAvailable ? (
            <PersonalInfoSection
              formData={formData}
              onChange={handleChange}
              errors={errors}
              groups={groups}
            />
          ) : null}

          {studentFormAvailable ? (
            <>
              {/* Guardian Information: add directly during creation. Styling
              matches the other modal sections (border-gray-100 + neutral
              surface) and reuses the guardian modal's own dashed add-button
              and red remove patterns. */}
              <section
                ref={guardianSectionRef}
                aria-label="Erziehungsberechtigte"
                className="scroll-mt-4 rounded-xl border border-gray-100 bg-gray-50 p-3 md:p-4"
              >
                <div className="mb-3 flex items-center justify-between md:mb-4">
                  <h3 className="flex items-center gap-2 text-xs font-semibold text-gray-900 md:text-sm">
                    <MotoConceptIcon concept="groups" size={18} />
                    Erziehungsberechtigte
                  </h3>
                  <span className="text-xs text-gray-500">
                    {guardians.length > 0
                      ? `${guardians.length} hinzugefügt`
                      : "Optional"}
                  </span>
                </div>

                {guardians.length > 0 && (
                  <ul className="mb-3 space-y-2">
                    {guardians.map((guardian, index) => {
                      const flags = [
                        guardian.can_pickup ? "Abholberechtigt" : null,
                        guardian.is_emergency_contact ? "Notfallkontakt" : null,
                        guardian.is_primary ? "Primär" : null,
                      ].filter(Boolean);
                      return (
                        <li
                          key={`${guardian.first_name}-${guardian.last_name}-${guardian.email ?? ""}`}
                          className="flex items-start justify-between gap-2 rounded-lg border border-gray-100 bg-white p-2 md:p-3"
                        >
                          <div className="min-w-0">
                            <p className="truncate text-xs font-medium text-gray-900 md:text-sm">
                              {`${guardian.first_name} ${guardian.last_name}`.trim() ||
                                "Erziehungsberechtigte/r"}
                              <span className="ml-2 font-normal text-gray-500">
                                {relationshipLabel(guardian.relationship_type)}
                              </span>
                            </p>
                            {flags.length > 0 && (
                              <p className="mt-0.5 truncate text-xs text-gray-500">
                                {flags.join(" · ")}
                              </p>
                            )}
                          </div>
                          <button
                            type="button"
                            onClick={() => removeGuardian(index)}
                            disabled={saveLoading}
                            aria-label="Erziehungsberechtigte/n entfernen"
                            className="flex flex-shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs text-red-600 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                )}

                {guardianPickerOpen ? (
                  // Existing-guardian path is inline (not a second modal): a search
                  // is a light lookup, so it expands here in place. "Neu anlegen"
                  // stays a full modal because the new-guardian form is heavy.
                  <GuardianPickerPanel
                    onSelect={handleGuardianPick}
                    onCancel={handlePickerCancel}
                    excludeProfileIds={addedProfileIds}
                  />
                ) : (
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <button
                      type="button"
                      onClick={() => setGuardianModalOpen(true)}
                      disabled={saveLoading}
                      className="flex flex-1 items-center justify-center gap-2 rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 py-2 text-xs font-medium text-gray-600 transition-all duration-200 hover:border-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
                    >
                      <Plus className="h-4 w-4" />
                      Neu anlegen
                    </button>
                    <button
                      type="button"
                      onClick={() => setGuardianPickerOpen(true)}
                      disabled={saveLoading}
                      className="flex flex-1 items-center justify-center gap-2 rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 py-2 text-xs font-medium text-gray-600 transition-all duration-200 hover:border-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
                    >
                      <Search className="h-4 w-4" />
                      Vorhandene/n suchen
                    </button>
                  </div>
                )}
              </section>

              {/* Betreuungszeiten — weekly arrival/pickup times staged here and
              persisted atomically with the student, mirroring the guardian
              section above. Reuses the existing CareWeeklyPlanModal editor. */}
              <section
                aria-label="Betreuungszeiten"
                className="rounded-xl border border-gray-100 bg-gray-50 p-3 md:p-4"
              >
                <div className="mb-3 flex items-center justify-between md:mb-4">
                  <h3 className="flex items-center gap-2 text-xs font-semibold text-gray-900 md:text-sm">
                    <MotoConceptIcon concept="careTimes" size={16} />
                    Betreuungszeiten
                  </h3>
                  <span className="text-xs text-gray-500">
                    {hasCarePlan
                      ? `${scheduledWeekdays.length} Tage`
                      : "Optional"}
                  </span>
                </div>

                {hasCarePlan && (
                  <div className="mb-3 flex items-start justify-between gap-2 rounded-lg border border-gray-100 bg-white p-2 md:p-3">
                    <div className="min-w-0">
                      <p className="truncate text-xs font-medium text-gray-900 md:text-sm">
                        {scheduledWeekdays.map(weekdayShort).join(" · ")}
                      </p>
                      <p className="mt-0.5 truncate text-xs text-gray-500">
                        {`${arrivalSchedules.length}× Ankunft · ${pickupSchedules.length}× Abholung`}
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => {
                        setArrivalSchedules([]);
                        setPickupSchedules([]);
                      }}
                      disabled={saveLoading}
                      aria-label="Betreuungszeiten entfernen"
                      className="flex flex-shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs text-red-600 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                )}

                <button
                  type="button"
                  onClick={handleOpenCarePlan}
                  disabled={saveLoading}
                  className="flex w-full items-center justify-center gap-2 rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 py-2 text-xs font-medium text-gray-600 transition-all duration-200 hover:border-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
                >
                  <Plus className="h-4 w-4" />
                  {hasCarePlan
                    ? "Wochenplan bearbeiten"
                    : "Wochenplan hinzufügen"}
                </button>
              </section>

              {/* Common Form Sections */}
              <StudentCommonFormSections
                formData={formData}
                errors={errors}
                onChange={handleChange}
              />

              {/* How the child leaves each weekday (alleine / Bus / Abholung) */}
              <DepartureSection
                days={formData.allowed_departure_modes}
                onChange={(value) => {
                  const allowed = normalizeAllowedDepartureModes(value);
                  const departure = allowedDepartureToDepartureDays(allowed);
                  const busDays = allowedDepartureToBusDays(allowed);
                  const pickupDays = allowedDepartureToPickupDays(allowed);
                  setFormData((prev) => ({
                    ...prev,
                    allowed_departure_modes: allowed,
                    departure_days: departure,
                    bus_days: busDays,
                    bus: busDaysHaveAny(busDays),
                    pickup_days: pickupDays,
                    pickup_status: pickupDaysHaveAny(pickupDays)
                      ? "Wird abgeholt"
                      : "Geht alleine nach Hause",
                  }));
                }}
                companionNote={formData.departure_companion_note}
                onCompanionNoteChange={(value) =>
                  setFormData((prev) => ({
                    ...prev,
                    departure_companion_note: value,
                  }))
                }
                companionNoteError={errors.departure_companion_note}
              />
            </>
          ) : null}

          {/* Action Buttons */}
          <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
            <button
              type="button"
              onClick={onClose}
              disabled={saveLoading}
              className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
            >
              Abbrechen
            </button>
            {mode === "list-entry" || studentFormAvailable ? (
              <button
                type="submit"
                disabled={saveLoading}
                className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
              >
                {saveLoading ? (
                  <span className="flex items-center justify-center gap-2">
                    <svg
                      className="h-4 w-4 animate-spin text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                    >
                      <circle
                        className="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        strokeWidth="4"
                      ></circle>
                      <path
                        className="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      ></path>
                    </svg>
                    Wird erstellt...
                  </span>
                ) : (
                  "Erstellen"
                )}
              </button>
            ) : null}
          </div>
        </form>
      </Modal>

      {/* Reuse the existing guardian form: collect guardians without firing
          API calls — they are persisted atomically with the student. */}
      {guardianModalOpen && (
        <GuardianFormModal
          isOpen={guardianModalOpen}
          mode="create"
          onClose={() => setGuardianModalOpen(false)}
          onSubmit={handleGuardianSubmit}
        />
      )}

      {/* Reuse the existing weekly care-plan editor. onSubmit only stages the
          plan locally (no API calls) — it is persisted atomically with the
          student on create. successMessage overrides the default "saved"
          wording since nothing is persisted yet here. */}
      {carePlanModalOpen && careDaysSource && (
        <CareWeeklyPlanModal
          isOpen={carePlanModalOpen}
          careDaysSource={careDaysSource}
          onClose={() => setCarePlanModalOpen(false)}
          initialArrivalSchedules={arrivalSchedules}
          initialPickupSchedules={pickupSchedules}
          successMessage="Betreuungszeiten übernommen"
          onSubmit={async ({ arrivalSchedules: nextArrival, pickupData }) => {
            setArrivalSchedules(nextArrival);
            setPickupSchedules(pickupData.schedules);
          }}
        />
      )}
    </>
  );
}
