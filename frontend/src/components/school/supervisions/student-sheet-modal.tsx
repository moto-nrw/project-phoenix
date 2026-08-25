"use client";

// Kind-Infoblatt der Aufsicht (#2527): Abholplan des Tages, wer das Kind
// abholen darf und wen man im Notfall anruft.
//
// Absichtlich hinter einem Antippen und nicht in der Liste: Telefonnummern
// der Familien gehören nicht dauerhaft auf einen offenen Bildschirm, und
// jeder Aufruf schreibt serverseitig eine Zugriffszeile. Das Backend gibt
// das Blatt nur für Kinder der eigenen Aufsicht heraus.

import { useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { Modal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { getRelationshipTypeLabel } from "~/lib/guardian-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { schoolClassLabel } from "~/lib/school-class-label";
import {
  schoolSupervisionsApi,
  type SupervisionContact,
  type SupervisionStudentSheet,
} from "~/lib/school-supervisions-api";

const logger = createLogger({ component: "SchoolStudentSheetModal" });

const STATUS_LABELS: Record<string, string> = {
  sick: "Krank gemeldet",
  excused: "Entschuldigt",
  class_trip: "Klassenfahrt",
  cancelled: "Heute abgemeldet",
};

const STATUS_COLORS: Record<string, string> = {
  sick: LOCATION_COLORS.SICK,
  excused: LOCATION_COLORS.EXCUSED,
  class_trip: LOCATION_COLORS.CLASS_TRIP,
  cancelled: LOCATION_COLORS.UNKNOWN,
};

const NO_ENTRY = "Keine Angabe";

function ContactRows({
  contacts,
  emptyText,
  showNote,
}: Readonly<{
  contacts: SupervisionContact[];
  emptyText: string;
  showNote: boolean;
}>) {
  if (contacts.length === 0) {
    return <p className="text-xs text-gray-500 md:text-sm">{emptyText}</p>;
  }
  return (
    <DataGrid>
      {contacts.map((contact, index) => (
        <DataField
          key={`${contact.name}-${contact.phones.join("-") || index}`}
          label={
            contact.relationship
              ? getRelationshipTypeLabel(contact.relationship)
              : "Kontakt"
          }
        >
          <span className="block">{contact.name}</span>
          {/* Jede hinterlegte Nummer bekommt ihren eigenen Link: ein Link mit
              zwei Nummern wählt keine davon. */}
          {contact.phones.length > 0 ? (
            contact.phones.map((phone) => (
              <a
                key={phone}
                href={`tel:${phone.replace(/[^\d+]/g, "")}`}
                className="mt-0.5 block underline decoration-gray-300 underline-offset-4"
              >
                {phone}
              </a>
            ))
          ) : (
            <span className="mt-0.5 block text-xs font-normal text-gray-500">
              Keine Telefonnummer hinterlegt
            </span>
          )}
          {showNote && contact.note ? (
            <span className="mt-0.5 block text-xs font-normal text-gray-600">
              {contact.note}
            </span>
          ) : null}
        </DataField>
      ))}
    </DataGrid>
  );
}

export interface StudentSheetModalProps {
  readonly instanceId: string;
  readonly studentId: string | null;
  readonly studentName: string;
  readonly onClose: () => void;
}

export function StudentSheetModal({
  instanceId,
  studentId,
  studentName,
  onClose,
}: StudentSheetModalProps) {
  const [sheet, setSheet] = useState<SupervisionStudentSheet | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!studentId) return;
    let cancelled = false;
    setSheet(null);
    setFailed(false);
    schoolSupervisionsApi
      .studentSheet(instanceId, studentId)
      .then((result) => {
        if (!cancelled) setSheet(result);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setFailed(true);
        logger.error("student_sheet_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [instanceId, studentId]);

  const title = sheet
    ? `${sheet.firstName} ${sheet.lastName}`.trim()
    : studentName;

  return (
    <Modal isOpen={studentId !== null} onClose={onClose} title={title}>
      <div className="space-y-4">
        {failed ? (
          <Alert
            type="error"
            message="Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal."
          />
        ) : null}
        {!sheet && !failed ? (
          <div className="space-y-3">
            <Skeleton className="h-20 w-full" />
            <Skeleton className="h-28 w-full" />
          </div>
        ) : null}
        {sheet ? (
          <>
            <div className="flex flex-wrap items-center gap-2">
              {sheet.schoolClass ? (
                <span className="text-sm text-gray-600">
                  {schoolClassLabel(sheet.schoolClass)}
                </span>
              ) : null}
              {sheet.status ? (
                <StatusDotBadge
                  label={STATUS_LABELS[sheet.status] ?? sheet.status}
                  color={STATUS_COLORS[sheet.status] ?? LOCATION_COLORS.UNKNOWN}
                />
              ) : null}
            </div>

            <InfoSection
              title="Heute"
              icon={<MotoConceptIcon concept="classDay" size={16} />}
            >
              {/* DataField statt StatTile: die InfoSection-Fläche ist selbst
                  schon gray-50, eine graue Kachel darauf verschwindet. */}
              <DataGrid>
                <DataField label="Kommt um">
                  {sheet.arrival ?? NO_ENTRY}
                </DataField>
                <DataField label="Geht um">
                  {sheet.pickup ?? NO_ENTRY}
                </DataField>
                <DataField label="Geht so nach Hause" fullWidth>
                  {sheet.departure || NO_ENTRY}
                </DataField>
              </DataGrid>
            </InfoSection>

            <InfoSection
              title="Darf abholen"
              icon={<MotoConceptIcon concept="pickup" size={16} />}
            >
              <ContactRows
                contacts={sheet.pickupContacts}
                showNote
                emptyText="Für dieses Kind ist niemand zum Abholen hinterlegt. Bitte im OGS-Büro nachfragen."
              />
            </InfoSection>

            <InfoSection
              title="Im Notfall anrufen"
              icon={<MotoConceptIcon concept="emergency" size={16} />}
            >
              <ContactRows
                contacts={sheet.emergencyContacts}
                showNote={false}
                emptyText="Für dieses Kind ist kein Notfallkontakt hinterlegt. Bitte im OGS-Büro nachfragen."
              />
            </InfoSection>

            <p className="text-xs text-gray-500">
              Diese Angaben pflegt das OGS-Büro. Änderungen bitte dort melden.
            </p>
          </>
        ) : null}
      </div>
    </Modal>
  );
}
