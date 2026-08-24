"use client";

// Kind-Infoblatt der Aufsicht (#2527): Abholplan des Tages, wer das Kind
// abholen darf und wen man im Notfall anruft.
//
// Absichtlich hinter einem Antippen und nicht in der Liste: Telefonnummern
// der Familien gehören nicht dauerhaft auf einen offenen Bildschirm, und
// jeder Aufruf schreibt serverseitig eine Zugriffszeile. Das Backend gibt
// das Blatt nur für Kinder der eigenen Aufsicht heraus.

import { Phone } from "lucide-react";
import { useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Modal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { getRelationshipTypeLabel } from "~/lib/guardian-helpers";
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

function Fact({
  label,
  value,
}: Readonly<{ label: string; value: string | undefined }>) {
  return (
    <div className="rounded-xl bg-gray-50 px-3 py-2">
      <span className="block text-sm font-semibold text-gray-900">
        {value && value !== "" ? value : "Keine Angabe"}
      </span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}

function ContactList({
  contacts,
  emptyText,
}: Readonly<{ contacts: SupervisionContact[]; emptyText: string }>) {
  if (contacts.length === 0) {
    return <p className="text-sm text-gray-500">{emptyText}</p>;
  }
  return (
    <ul className="space-y-1.5">
      {contacts.map((contact, index) => (
        <li
          key={`${contact.name}-${contact.phone ?? index}`}
          className="rounded-xl border border-gray-100 bg-white px-3 py-2.5"
        >
          <p className="text-sm font-medium text-gray-900">{contact.name}</p>
          {contact.relationship ? (
            <p className="text-xs text-gray-500">
              {getRelationshipTypeLabel(contact.relationship)}
            </p>
          ) : null}
          {contact.phone ? (
            <a
              href={`tel:${contact.phone.replace(/\s/g, "")}`}
              className="mt-1 inline-flex items-center gap-1.5 text-sm font-medium text-gray-900 underline decoration-gray-300 underline-offset-4"
            >
              <Phone className="h-3.5 w-3.5" aria-hidden="true" />
              {contact.phone}
            </a>
          ) : (
            <p className="mt-1 text-xs text-gray-500">
              Keine Telefonnummer hinterlegt
            </p>
          )}
          {contact.note ? (
            <p className="mt-1 text-xs text-gray-600">{contact.note}</p>
          ) : null}
        </li>
      ))}
    </ul>
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
      <div className="space-y-5">
        {failed ? (
          <Alert
            type="error"
            message="Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal."
          />
        ) : null}
        {!sheet && !failed ? (
          <div className="space-y-3">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-24 w-full" />
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

            <div>
              <h3 className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Heute
              </h3>
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                <Fact label="Kommt um" value={sheet.arrival} />
                <Fact label="Geht um" value={sheet.pickup} />
                <Fact label="Geht so nach Hause" value={sheet.departure} />
              </div>
            </div>

            <div>
              <h3 className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Darf abholen
              </h3>
              <ContactList
                contacts={sheet.pickupContacts}
                emptyText="Für dieses Kind ist niemand zum Abholen hinterlegt. Bitte im OGS-Büro nachfragen."
              />
            </div>

            <div>
              <h3 className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Im Notfall anrufen
              </h3>
              <ContactList
                contacts={sheet.emergencyContacts}
                emptyText="Für dieses Kind ist kein Notfallkontakt hinterlegt. Bitte im OGS-Büro nachfragen."
              />
            </div>

            <p className="text-xs text-gray-500">
              Diese Angaben pflegt das OGS-Büro. Änderungen bitte dort melden.
            </p>
          </>
        ) : null}
      </div>
    </Modal>
  );
}
