"use client";

import { Alert } from "~/components/ui/alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { EnrollmentRequestUploadList } from "~/components/enrollment/admin-enrollment-upload-list";
import { FeaturePill } from "~/components/enrollment/feature-pill";
import type {
  AdminRequestChild,
  AdminRequestChildOffering,
  AdminRequestSchemaField,
  AdminRequestSummary,
} from "~/lib/enrollment-admin-api";
import { formatOfferingPrice } from "~/lib/care-offering-format";
import { formatCustomValue } from "~/lib/enrollment-custom-value-format";
import { formatCalendarDate } from "~/lib/localized-date-format";
import { childrenConcept } from "~/lib/concepts/children";
import { parentsConcept } from "~/lib/concepts/parents";
import { weekdayShortList } from "~/lib/weekdays";

export function formatDateTime(
  value: string,
  options?: Intl.DateTimeFormatOptions,
): string {
  return new Date(value).toLocaleString("de-DE", {
    ...options,
    timeZone: "Europe/Berlin",
  });
}

export function formatPlainDate(value: string): string {
  return formatCalendarDate(value, "de-DE");
}

const CONSENT_LABELS: Record<string, string> = {
  agb: "AGB der Schule",
  data_processing: "Datenverarbeitung (DSGVO)",
  email_contact: "E-Mail-Kontakt",
  photo: "Fotos bei Schulveranstaltungen",
};

export function RequestExtraSection({
  request,
}: Readonly<{ request: AdminRequestSummary }>) {
  const guardianFields = (request.schema_fields ?? []).filter(
    (f) => !f.applies_to_child,
  );
  const hasCustom = guardianFields.some(
    (f) => formatCustomValue(request.custom_data?.[f.key], f) !== null,
  );
  const hasConsents = Object.keys(request.consent_flags ?? {}).length > 0;
  // Titles from the pinned schema's legal blocks label custom consents
  // (e.g. "Schwimmbad" instead of "custom_pool"); the static map covers
  // the standard keys for legacy requests without a pinned schema.
  const consentTitles = new Map(
    (request.schema_legal_blocks ?? []).map((block) => [
      block.key,
      block.title,
    ]),
  );

  if (!hasCustom && !hasConsents) return null;

  return (
    <section className="moto-content-surface space-y-3 rounded-2xl border p-4 sm:p-6">
      <h2 className="text-base font-semibold text-gray-900">
        Zusätzliche Angaben
      </h2>

      {hasConsents && (
        <div>
          <h3 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
            Zustimmungen
          </h3>
          <ul className="mt-1.5 space-y-1 text-sm">
            {Object.entries(request.consent_flags ?? {}).map(([key, val]) => (
              <li key={key} className="flex items-center gap-2">
                <span className="text-gray-700">
                  {consentTitles.get(key) ?? CONSENT_LABELS[key] ?? key}:
                </span>
                <span className="font-medium text-gray-900">
                  {val === true ? "Ja" : val === false ? "Nein" : String(val)}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {hasCustom && (
        <InfoSection
          title="Zusatzfragen (Eltern)"
          icon={<MotoConceptIcon concept={parentsConcept} size={16} />}
        >
          <DataGrid>
            {guardianFields.map((f) => {
              const formatted = formatCustomValue(
                request.custom_data?.[f.key],
                f,
              );
              if (formatted === null) return null;
              return (
                <DataField
                  key={f.key}
                  label={f.label}
                  fullWidth={f.type === "file"}
                >
                  {f.type === "file" ? (
                    <EnrollmentRequestUploadList
                      requestId={request.id}
                      value={request.custom_data?.[f.key]}
                      uploads={request.uploads}
                      canView={request.can_view_uploads}
                    />
                  ) : (
                    formatted
                  )}
                </DataField>
              );
            })}
          </DataGrid>
        </InfoSection>
      )}
    </section>
  );
}

// ChildOfferings renders the per-child Betreuungsangebote selection.
// For fixed offerings the day badge shows the offering's full
// schedule (Mo, Di, …). For parent_choice offerings the badge marks
// only the days the parent picked, prefixed with the picked count
// so admins can spot half-week selections at a glance.
export function ChildOfferings({
  offerings,
  upcomingOfferings,
  unavailable,
  phaseName,
}: Readonly<{
  offerings?: AdminRequestChildOffering[];
  /** Rendered alongside the current selection, flagged "gilt ab …". */
  upcomingOfferings?: AdminRequestChildOffering[];
  /**
   * The lookup failed. Rendered as a warning rather than nothing: staff
   * decide on this screen, and an absent block reads as "the family booked
   * nothing" — which is how the wrong decision gets made (#2185).
   */
  unavailable?: boolean;
  phaseName?: string;
}>) {
  const rows = [...(offerings ?? []), ...(upcomingOfferings ?? [])];
  if (unavailable) {
    return (
      <Alert
        type="warning"
        announce="off"
        title="Betreuungsangebote"
        message="Die gebuchten Angebote konnten nicht geladen werden. Bitte die Seite neu laden, bevor Sie über dieses Kind entscheiden."
        className="mt-3"
      />
    );
  }
  if (rows.length === 0) return null;
  return (
    <InsetPanel framed className="mt-3">
      <h4 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
        Betreuungsangebote
      </h4>
      <ul className="mt-1.5 space-y-2 text-sm">
        {rows.map((o) => {
          const parentChoice = o.days_of_week_mode === "parent_choice";
          const dayDetails = parentChoice
            ? formatAdminOfferingDaySource(o)
            : weekdayShortList(o.available_days ?? []);
          return (
            <li key={`${o.offering_id}-${o.valid_from ?? "current"}`}>
              <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <span className="font-medium text-gray-900">
                  {o.offering_name || `Angebot #${o.offering_id}`}
                </span>
                {dayDetails ? (
                  <span className="text-xs text-gray-600">
                    {parentChoice ? dayDetails : `Tage: ${dayDetails}`}
                  </span>
                ) : parentChoice ? (
                  <span className="text-moto-red-strong text-xs italic">
                    Keine Tage gewählt
                  </span>
                ) : null}
              </div>
              <OfferingAttributePills offering={o} />
            </li>
          );
        })}
      </ul>
      {/* Where the attributes come from. The support case behind #2185 was a
          coordinator who could not place the "mit Mittagessen" the parents app
          showed — naming the phase turns the badges into something staff can
          act on instead of another mystery. */}
      <p className="mt-2 text-xs text-gray-500">
        Angaben stammen aus der Angebots-Konfiguration
        {phaseName
          ? ` der Anmeldephase „${phaseName}“`
          : " der Anmeldephase"}{" "}
        (Anmeldungen → Betreuungsangebote).
      </p>
    </InsetPanel>
  );
}

// The offering's attributes and validity, phrased exactly as the parents app
// phrases them for the same booking (#2185) — "mit Mittagessen", not
// "Mittagessen" — so a guardian quoting their screen and the staff member
// reading this row use the same words.
function OfferingAttributePills({
  offering,
}: Readonly<{ offering: AdminRequestChildOffering }>) {
  const price = formatOfferingPrice(offering.price_cents);
  const pills: string[] = [];
  if (offering.includes_lunch) pills.push("mit Mittagessen");
  if (offering.includes_holiday_care) pills.push("mit Ferienbetreuung");
  if (price) pills.push(`${price} pro Monat`);
  if (offering.valid_until) {
    // Already the inclusive last covered day on the wire.
    pills.push(`bis ${formatPlainDate(offering.valid_until)}`);
  }
  const startsLater = offering.starts_later === true && !!offering.valid_from;
  if (pills.length === 0 && !startsLater) return null;
  return (
    <div className="mt-1 flex flex-wrap items-center gap-1.5">
      {startsLater ? (
        <StatusBadge
          tone="blue"
          label={`gilt ab ${formatPlainDate(offering.valid_from ?? "")}`}
          title="Diese Buchung ist noch nicht wirksam, sie beginnt erst zum genannten Datum."
        />
      ) : null}
      {pills.map((label) => (
        <FeaturePill key={label} label={label} />
      ))}
    </div>
  );
}

function formatAdminOfferingDaySource(o: AdminRequestChildOffering): string {
  const automatic = weekdayShortList(o.automatic_selected_days ?? []);
  const manualDays =
    (o.manual_selected_days?.length ?? 0) > 0 ||
    (o.automatic_selected_days?.length ?? 0) > 0
      ? (o.manual_selected_days ?? [])
      : (o.selected_days ?? []);
  const manual = weekdayShortList(manualDays);
  if (automatic && manual) {
    return `${automatic} automatisch mitgebucht; ${manual} von Eltern gewählt`;
  }
  if (automatic) return `${automatic} automatisch mitgebucht`;
  if (manual) return `Von Eltern gewählt: ${manual}`;
  return "";
}

// Reserved child custom-data key carrying the coupled "mit wem" note for the
// accompanied departure mode (#1694). It rides alongside
// student.allowed_departure_modes instead of being a standalone schema field,
// so it is NOT in schema_fields and must be rendered explicitly here — the
// backend persists it onto the student on approval, so staff must see it before
// deciding. Matches DEPARTURE_COMPANION_KEY in enrollment-form.tsx and
// TargetStudentDepartureCompanionNote in the backend.
const DEPARTURE_COMPANION_KEY = "student.departure_companion_note";

export function ChildExtraFields({
  child,
  schemaFields,
}: Readonly<{
  child: AdminRequestChild;
  schemaFields?: AdminRequestSchemaField[];
}>) {
  const childFields = (schemaFields ?? []).filter((f) => f.applies_to_child);
  const filled = childFields
    .map((f) => ({
      field: f,
      value: formatCustomValue(child.custom_data?.[f.key], f),
    }))
    .filter((row) => row.value !== null);
  const companionNote = (
    child.custom_data?.[DEPARTURE_COMPANION_KEY] as string | undefined
  )?.trim();
  if (filled.length === 0 && !companionNote) return null;
  return (
    <div className="mt-3">
      <InfoSection
        title="Zusätzliche Angaben zum Kind"
        icon={<MotoConceptIcon concept={childrenConcept} size={16} />}
      >
        <DataGrid>
          {filled.map(({ field, value }) => (
            <DataField key={field.key} label={field.label}>
              {value}
            </DataField>
          ))}
          {companionNote && (
            <DataField key={DEPARTURE_COMPANION_KEY} label="Mit welchem Kind?">
              {companionNote}
            </DataField>
          )}
        </DataGrid>
      </InfoSection>
    </div>
  );
}
