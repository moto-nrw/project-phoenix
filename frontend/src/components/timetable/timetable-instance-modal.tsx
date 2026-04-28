"use client";

import { useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { fetchStudents } from "~/lib/student-api";
import { staffService } from "~/lib/staff-api";
import { timetableService } from "~/lib/timetable-api";
import type {
  CreateInstanceBody,
  EnrichedInstance,
} from "~/lib/timetable-types";

interface RoomOption {
  id: number;
  name: string;
  building?: string;
}

interface PersonOption {
  id: string;
  name: string;
}

interface BackendRoomsEnvelope {
  data?: Array<{
    id: number;
    name: string;
    building?: string;
  }>;
}

interface FormState {
  date: string;
  title: string;
  startTime: string;
  endTime: string;
  roomId: string;
  notes: string;
  studentIds: string[];
  staffIds: string[];
}

interface TimetableInstanceModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved: (instance: EnrichedInstance) => void;
  initialInstance?: EnrichedInstance | null;
  defaultDate: string;
}

const logger = createLogger({ component: "TimetableInstanceModal" });

function emptyForm(defaultDate: string): FormState {
  return {
    date: defaultDate,
    title: "",
    startTime: "12:00",
    endTime: "13:00",
    roomId: "",
    notes: "",
    studentIds: [],
    staffIds: [],
  };
}

export function TimetableInstanceModal({
  isOpen,
  onClose,
  onSaved,
  initialInstance = null,
  defaultDate,
}: TimetableInstanceModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [form, setForm] = useState<FormState>(() => emptyForm(defaultDate));
  const [rooms, setRooms] = useState<RoomOption[]>([]);
  const [students, setStudents] = useState<PersonOption[]>([]);
  const [staff, setStaff] = useState<PersonOption[]>([]);
  const [loadingRefs, setLoadingRefs] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setForm(
      initialInstance
        ? {
            date: initialInstance.date,
            title: initialInstance.title,
            startTime: initialInstance.startTime,
            endTime: initialInstance.endTime,
            roomId: initialInstance.roomId,
            notes: initialInstance.notes ?? "",
            studentIds: initialInstance.studentIds,
            staffIds: initialInstance.staff.map((item) => item.staffId),
          }
        : emptyForm(defaultDate),
    );
    setValidationError(null);
    setLoadingRefs(true);

    void Promise.all([
      fetch("/api/rooms", { credentials: "include" })
        .then((r) => r.json() as Promise<BackendRoomsEnvelope>)
        .then((j): RoomOption[] =>
          (j.data ?? []).map((room) => ({
            id: room.id,
            name: room.name,
            building: room.building,
          })),
        )
        .catch((err: unknown) => {
          logger.error("rooms_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as RoomOption[];
        }),
      fetchStudents({ page_size: 500 })
        .then((res) =>
          res.students.map((student) => ({
            id: student.id,
            name: student.name,
          })),
        )
        .catch((err: unknown) => {
          logger.error("students_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as PersonOption[];
        }),
      staffService
        .getAllStaff()
        .then((items) => items.map((s) => ({ id: s.id, name: s.name })))
        .catch((err: unknown) => {
          logger.error("staff_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as PersonOption[];
        }),
    ])
      .then(([roomData, studentData, staffData]) => {
        setRooms(
          [...roomData].sort((a, b) => a.name.localeCompare(b.name, "de")),
        );
        setStudents(studentData);
        setStaff(staffData);
      })
      .finally(() => setLoadingRefs(false));
  }, [defaultDate, initialInstance, isOpen]);

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setValidationError(null);
  };

  const canSubmit = useMemo(
    () =>
      form.date !== "" &&
      form.title.trim() !== "" &&
      form.startTime !== "" &&
      form.endTime !== "" &&
      form.roomId !== "" &&
      !submitting &&
      (!initialInstance || initialInstance.status === "planned"),
    [form, initialInstance, submitting],
  );

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!canSubmit) return;
    if (form.endTime <= form.startTime) {
      setValidationError("Endzeit muss nach der Startzeit liegen.");
      return;
    }
    const roomId = Number.parseInt(form.roomId, 10);
    if (!Number.isFinite(roomId) || roomId <= 0) {
      setValidationError("Bitte einen Raum auswählen.");
      return;
    }

    const body: CreateInstanceBody = {
      date: form.date,
      title: form.title.trim(),
      start_time: form.startTime,
      end_time: form.endTime,
      room_id: roomId,
      notes: form.notes.trim() || undefined,
      activity_group_id: initialInstance?.activityGroupId
        ? Number(initialInstance.activityGroupId)
        : undefined,
      staff_ids: form.staffIds.map(Number),
      student_ids: form.studentIds.map(Number),
    };

    setSubmitting(true);
    try {
      const saved = initialInstance
        ? await timetableService.update(initialInstance.id, body)
        : await timetableService.create(body);
      toastSuccess(initialInstance ? "Termin gespeichert" : "Termin angelegt");
      onSaved(saved);
      onClose();
    } catch (err) {
      logger.error("instance_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      const msg =
        err instanceof Error
          ? err.message
          : "Termin konnte nicht gespeichert werden";
      setValidationError(msg);
      toastError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  const isEditing = initialInstance !== null;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEditing ? "Termin bearbeiten" : "Termin hinzufügen"}
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="timetable-instance-form"
            variant="primary"
            size="sm"
            isLoading={submitting}
            loadingText="Speichere …"
            disabled={!canSubmit}
          >
            Speichern
          </Button>
        </div>
      }
    >
      <form
        id="timetable-instance-form"
        onSubmit={(e) => void handleSubmit(e)}
        className="flex flex-col gap-4"
      >
        {initialInstance && initialInstance.status !== "planned" && (
          <div className="rounded-md border border-[#FECACA] bg-[#FEF2F2] px-3 py-2 text-xs font-semibold text-[#7F1D1D]">
            Nur geplante Termine können bearbeitet werden.
          </div>
        )}

        <Field label="Titel" htmlFor="instance_title" required>
          <Input
            id="instance_title"
            value={form.title}
            onChange={(e) => update("title", e.target.value)}
            placeholder="z. B. Extra Lernzeit, Raumwechsel, Yoga AG"
            maxLength={255}
            autoFocus
            required
          />
        </Field>

        <div className="grid grid-cols-3 gap-3">
          <Field label="Datum" htmlFor="instance_date" required>
            <Input
              id="instance_date"
              type="date"
              value={form.date}
              onChange={(e) => update("date", e.target.value)}
              required
            />
          </Field>
          <Field label="Start" htmlFor="instance_start" required>
            <Input
              id="instance_start"
              type="time"
              value={form.startTime}
              onChange={(e) => update("startTime", e.target.value)}
              required
            />
          </Field>
          <Field label="Ende" htmlFor="instance_end" required>
            <Input
              id="instance_end"
              type="time"
              value={form.endTime}
              onChange={(e) => update("endTime", e.target.value)}
              required
            />
          </Field>
        </div>

        <Field label="Raum" htmlFor="instance_room" required>
          <select
            id="instance_room"
            value={form.roomId}
            onChange={(e) => update("roomId", e.target.value)}
            disabled={loadingRefs}
            required
            className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-100"
          >
            <option value="">
              {loadingRefs ? "Lade Räume …" : "Raum auswählen …"}
            </option>
            {rooms.map((room) => (
              <option key={room.id} value={room.id}>
                {room.building ? `${room.building} – ${room.name}` : room.name}
              </option>
            ))}
          </select>
        </Field>

        <MultiSelectField
          label="Kinder"
          options={students}
          value={form.studentIds}
          onChange={(ids) => update("studentIds", ids)}
        />
        <MultiSelectField
          label="Personal"
          options={staff}
          value={form.staffIds}
          onChange={(ids) => update("staffIds", ids)}
        />

        <Field label="Notiz" htmlFor="instance_notes">
          <textarea
            id="instance_notes"
            value={form.notes}
            onChange={(e) => update("notes", e.target.value)}
            rows={3}
            className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none"
          />
        </Field>

        {validationError && (
          <div
            role="alert"
            className="rounded-md border border-[#FECACA] bg-[#FEF2F2] px-3 py-2 text-xs font-semibold text-[#7F1D1D]"
          >
            {validationError}
          </div>
        )}
      </form>
    </Modal>
  );
}

function Field({
  label,
  htmlFor,
  required = false,
  children,
}: {
  label: string;
  htmlFor: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <label htmlFor={htmlFor} className="flex flex-col gap-1">
      <span className="text-xs font-semibold text-slate-700">
        {label}
        {required && <span className="ml-0.5 text-[#FF3130]">*</span>}
      </span>
      {children}
    </label>
  );
}

function MultiSelectField({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: PersonOption[];
  value: string[];
  onChange: (value: string[]) => void;
}) {
  const selected = new Set(value);
  const toggle = (id: string) => {
    const next = selected.has(id)
      ? value.filter((item) => item !== id)
      : [...value, id];
    onChange(next);
  };

  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-semibold text-slate-700">{label}</span>
      <div className="max-h-40 overflow-y-auto rounded-lg border border-slate-200 bg-white p-2">
        {options.length === 0 ? (
          <div className="px-2 py-3 text-xs text-slate-500">
            Keine Einträge gefunden
          </div>
        ) : (
          <div className="grid gap-1 sm:grid-cols-2">
            {options.map((option) => (
              <label
                key={option.id}
                className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm text-slate-700 hover:bg-slate-50"
              >
                <input
                  type="checkbox"
                  checked={selected.has(option.id)}
                  onChange={() => toggle(option.id)}
                  className="h-4 w-4 rounded border-slate-300 text-[#5080D8] focus:ring-[#5080D8]"
                />
                <span className="min-w-0 truncate">{option.name}</span>
              </label>
            ))}
          </div>
        )}
      </div>
      {value.length > 0 && (
        <span className="text-[11px] text-slate-500">
          {value.length} ausgewählt
        </span>
      )}
    </div>
  );
}
