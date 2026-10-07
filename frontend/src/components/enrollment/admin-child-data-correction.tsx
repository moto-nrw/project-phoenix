"use client";

import { useLayoutEffect, useRef, useState, type FormEvent } from "react";
import { Pencil } from "lucide-react";
import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
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
  const errors = useApiFormError();
  // „Wiederholen“ sendet den aktuellen Stand, nicht den vom Fehler.
  const latestSubmitRef = useRef<() => Promise<void>>(async () => undefined);

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
    errors.clear();
    setOpen(true);
  };

  const save = async () => {
    const parsedGrade = grade === "" ? undefined : Number(grade);
    if (parsedGrade !== undefined && !Number.isInteger(parsedGrade)) {
      errors.invalid("Die Klassenstufe muss eine ganze Zahl sein.", {
        target_grade_level: "Bitte geben Sie eine ganze Zahl ein.",
      });
      return;
    }
    if (!reason.trim()) {
      errors.invalid("Bitte geben Sie einen Grund für die Korrektur an.", {
        reason: "Bitte geben Sie einen Grund an.",
      });
      return;
    }
    setSaving(true);
    errors.clear();
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
      await errors.show(err, {
        object: "die Korrektur",
        retry: () => void latestSubmitRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };
  useLayoutEffect(() => {
    latestSubmitRef.current = save;
  });

  const submit = (event: FormEvent) => {
    event.preventDefault();
    void save();
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
        error={errors.error}
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
          onSubmit={submit}
        >
          <p className="text-sm leading-6 text-gray-600">
            Die Anmeldung bleibt die Quelle. Nach dem Speichern werden die
            verknüpften Stammdaten automatisch aktualisiert und die Änderung
            protokolliert.
          </p>
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              name="first_name"
              label="Vorname"
              error={errors.fieldError("first_name")}
              value={firstName}
              onChange={(event) => setFirstName(event.target.value)}
              required
            />
            <Input
              name="last_name"
              label="Nachname"
              error={errors.fieldError("last_name")}
              value={lastName}
              onChange={(event) => setLastName(event.target.value)}
              required
            />
          </div>
          <ISODatePicker
            id="correction-date-of-birth"
            controlSize="lg"
            label="Geburtsdatum"
            error={errors.fieldError("date_of_birth")}
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
              name="target_grade_level"
              label="Ziel-Klassenstufe"
              error={errors.fieldError("target_grade_level")}
              type="number"
              min={1}
              max={13}
              step={1}
              value={grade}
              onChange={(event) => setGrade(event.target.value)}
            />
            <Input
              name="target_school_class"
              label="Zielklasse (optional)"
              error={errors.fieldError("target_school_class")}
              value={schoolClass}
              onChange={(event) => setSchoolClass(event.target.value)}
              placeholder="z. B. 2a"
            />
          </div>
          <Textarea
            id={`correction-reason-${child.id}`}
            name="reason"
            label="Grund der Korrektur"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            rows={3}
            required
            placeholder="z. B. Klassenstufe laut Rücksprache mit den Eltern berichtigt"
            error={errors.fieldError("reason")}
          />
        </form>
      </FormModal>
    </>
  );
}
