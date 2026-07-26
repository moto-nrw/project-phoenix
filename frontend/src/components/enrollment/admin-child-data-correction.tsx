"use client";

import { useState, type FormEvent } from "react";
import { Pencil } from "lucide-react";
import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { todayISO } from "~/lib/date-helpers";
import {
  correctAdminChildData,
  type AdminRequestChild,
} from "~/lib/enrollment-admin-api";

export function AdminChildDataCorrection({
  child,
  requestId,
  onSaved,
}: Readonly<{
  child: AdminRequestChild;
  requestId: string;
  onSaved: (correctedChild: AdminRequestChild) => void;
}>) {
  const [open, setOpen] = useState(false);
  const [firstName, setFirstName] = useState(child.first_name);
  const [lastName, setLastName] = useState(child.last_name);
  const [dateOfBirth, setDateOfBirth] = useState(child.date_of_birth);
  const [grade, setGrade] = useState(
    child.target_grade_level?.toString() ?? "",
  );
  const [schoolClass, setSchoolClass] = useState(
    child.target_school_class ?? "",
  );
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const close = () => {
    if (!saving) setOpen(false);
  };

  const openCorrection = () => {
    setFirstName(child.first_name);
    setLastName(child.last_name);
    setDateOfBirth(child.date_of_birth);
    setGrade(child.target_grade_level?.toString() ?? "");
    setSchoolClass(child.target_school_class ?? "");
    setReason("");
    setError(null);
    setOpen(true);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const parsedGrade = grade === "" ? undefined : Number(grade);
    if (parsedGrade !== undefined && !Number.isInteger(parsedGrade)) {
      setError("Die Klassenstufe muss eine ganze Zahl sein.");
      return;
    }
    if (!reason.trim()) {
      setError("Bitte gib einen Grund für die Korrektur an.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const correction = {
        first_name: firstName.trim(),
        last_name: lastName.trim(),
        date_of_birth: dateOfBirth,
        target_grade_level: parsedGrade,
        target_school_class: schoolClass.trim() || undefined,
        reason: reason.trim(),
      };
      const result = await correctAdminChildData(
        requestId,
        child.id,
        correction,
      );
      const correctedChild = result.request.children.find(
        (candidate) => candidate.id === child.id,
      ) ?? {
        ...child,
        first_name: correction.first_name,
        last_name: correction.last_name,
        date_of_birth: correction.date_of_birth,
        target_grade_level: correction.target_grade_level,
        target_school_class: correction.target_school_class,
      };
      setOpen(false);
      onSaved(correctedChild);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unbekannter Fehler");
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={openCorrection}
      >
        <Pencil className="mr-2 h-4 w-4" aria-hidden="true" />
        Anmeldedaten korrigieren
      </Button>
      <FormModal
        isOpen={open}
        onClose={close}
        title="Anmeldedaten korrigieren"
        size="sm"
        mobilePosition="center"
        footer={
          <>
            <Button
              type="button"
              variant="ghost"
              size="md"
              onClick={close}
              disabled={saving}
            >
              Abbrechen
            </Button>
            <Button
              type="submit"
              form={`child-data-correction-${child.id}`}
              size="md"
              isLoading={saving}
              loadingText="Speichert…"
            >
              Korrektur speichern
            </Button>
          </>
        }
      >
        <form
          id={`child-data-correction-${child.id}`}
          className="space-y-4"
          onSubmit={(event) => void submit(event)}
        >
          <p className="text-sm leading-6 text-gray-600">
            Die Anmeldung bleibt die Quelle. Nach dem Speichern werden die
            verknüpften Stammdaten automatisch aktualisiert und die Änderung
            protokolliert.
          </p>
          {error ? (
            <p
              className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]"
              role="alert"
            >
              {error}
            </p>
          ) : null}
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              name="correction-first-name"
              label="Vorname"
              value={firstName}
              onChange={(event) => setFirstName(event.target.value)}
              required
            />
            <Input
              name="correction-last-name"
              label="Nachname"
              value={lastName}
              onChange={(event) => setLastName(event.target.value)}
              required
            />
          </div>
          <ISODatePicker
            id="correction-date-of-birth"
            controlSize="lg"
            label="Geburtsdatum"
            value={dateOfBirth}
            onChange={setDateOfBirth}
            monthYearNavigation
            max={todayISO()}
            calendarLayout="popover"
            // submit() never validated the date itself, it relied on the native
            // `required`. The value always arrives set (date_of_birth is
            // mandatory on the enrollment record) and can no longer be cleared,
            // so an empty date stays impossible.
            hideClearButton
            required
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              name="correction-grade"
              label="Ziel-Klassenstufe"
              type="number"
              min={1}
              max={13}
              step={1}
              value={grade}
              onChange={(event) => setGrade(event.target.value)}
            />
            <Input
              name="correction-school-class"
              label="Zielklasse (optional)"
              value={schoolClass}
              onChange={(event) => setSchoolClass(event.target.value)}
              placeholder="z. B. 2a"
            />
          </div>
          <label htmlFor={`correction-reason-${child.id}`} className="block">
            <span className="mb-2 block text-sm font-medium text-gray-700">
              Grund der Korrektur
            </span>
            <textarea
              id={`correction-reason-${child.id}`}
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              rows={3}
              required
              placeholder="z. B. Klassenstufe laut Rücksprache mit den Eltern berichtigt"
              className="block w-full rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
            />
          </label>
        </form>
      </FormModal>
    </>
  );
}
