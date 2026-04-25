"use client";

/**
 * SpontaneousInstanceModal — form for adding a one-off planned activity
 * to the weekly grid. Spawned from the "+ Spontane Aktivität" button on
 * /database/timetables.
 *
 * Submits to POST /api/timetable/instances; the backend writes a row with
 * is_spontaneous=true (no template binding) and status=planned. The
 * created row drops straight into the SWR cache so the new card appears
 * on the grid without a manual refresh.
 *
 * Room list is fetched via the existing roomService — no new endpoint
 * needed.
 */

import { useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { roomService, type Room } from "~/lib/api";
import { createLogger } from "~/lib/logger";
import { timetableService } from "~/lib/timetable-api";
import type { EnrichedInstance } from "~/lib/timetable-types";

const logger = createLogger({ component: "SpontaneousInstanceModal" });

interface SpontaneousInstanceModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: (instance: EnrichedInstance) => void;
  /** Pre-fill the date when opened from a specific day. Defaults to today. */
  defaultDate?: string; // YYYY-MM-DD
}

interface FormState {
  date: string;
  startTime: string;
  endTime: string;
  title: string;
  roomId: string; // string for <select>, parsed to number on submit
  notes: string;
}

function todayISO(): string {
  const d = new Date();
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

function nextHalfHour(): string {
  const d = new Date();
  // Round forward to the next 30-minute mark.
  const minutes = d.getMinutes();
  const roundedMinutes = minutes < 30 ? 30 : 0;
  const hour = minutes < 30 ? d.getHours() : (d.getHours() + 1) % 24;
  const hh = String(hour).padStart(2, "0");
  const mm = String(roundedMinutes).padStart(2, "0");
  return `${hh}:${mm}`;
}

function plusOneHour(hhmm: string): string {
  const [hStr, mStr] = hhmm.split(":");
  if (!hStr || !mStr) return hhmm;
  const newHour = (Number.parseInt(hStr, 10) + 1) % 24;
  return `${String(newHour).padStart(2, "0")}:${mStr}`;
}

function emptyForm(defaultDate?: string): FormState {
  const start = nextHalfHour();
  return {
    date: defaultDate ?? todayISO(),
    startTime: start,
    endTime: plusOneHour(start),
    title: "",
    roomId: "",
    notes: "",
  };
}

export function SpontaneousInstanceModal({
  isOpen,
  onClose,
  onCreated,
  defaultDate,
}: SpontaneousInstanceModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();

  const [form, setForm] = useState<FormState>(() => emptyForm(defaultDate));
  const [rooms, setRooms] = useState<Room[]>([]);
  const [roomsLoading, setRoomsLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  // Reset the form whenever the modal opens so a previous draft does not
  // leak into the next session. Re-fetch rooms in case new ones were added.
  useEffect(() => {
    if (!isOpen) return;
    setForm(emptyForm(defaultDate));
    setValidationError(null);
    setRoomsLoading(true);
    roomService
      .getRooms()
      .then((data) => {
        // Sort alphabetically for the dropdown.
        const sorted = [...data].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        setRooms(sorted);
      })
      .catch((err: unknown) => {
        logger.error("rooms_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toastError("Räume konnten nicht geladen werden");
      })
      .finally(() => setRoomsLoading(false));
  }, [isOpen, defaultDate, toastError]);

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setValidationError(null);
  };

  const canSubmit = useMemo(() => {
    return (
      form.title.trim().length > 0 &&
      form.roomId !== "" &&
      form.date !== "" &&
      form.startTime !== "" &&
      form.endTime !== "" &&
      !submitting
    );
  }, [form, submitting]);

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

    setSubmitting(true);
    try {
      const created = await timetableService.create({
        date: form.date,
        start_time: form.startTime,
        end_time: form.endTime,
        title: form.title.trim(),
        room_id: roomId,
        notes: form.notes.trim() ? form.notes.trim() : undefined,
      });
      toastSuccess(`Aktivität "${created.title}" eingeplant`);
      onCreated(created);
      onClose();
    } catch (err) {
      logger.error("create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      const msg =
        err instanceof Error
          ? err.message
          : "Aktivität konnte nicht angelegt werden";
      setValidationError(msg);
      toastError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Spontane Aktivität einplanen"
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
            form="spontaneous-instance-form"
            variant="primary"
            size="sm"
            isLoading={submitting}
            loadingText="Lege an …"
            disabled={!canSubmit}
          >
            Aktivität anlegen
          </Button>
        </div>
      }
    >
      <form
        id="spontaneous-instance-form"
        onSubmit={(e) => void handleSubmit(e)}
        className="flex flex-col gap-4"
      >
        <Field label="Titel" htmlFor="title" required>
          <Input
            id="title"
            name="title"
            value={form.title}
            onChange={(e) => update("title", e.target.value)}
            placeholder="z. B. Bastelstunde, Yoga-Probe …"
            maxLength={255}
            required
            autoFocus
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Datum" htmlFor="date" required>
            <Input
              id="date"
              name="date"
              type="date"
              value={form.date}
              onChange={(e) => update("date", e.target.value)}
              required
            />
          </Field>
          <Field label="Raum" htmlFor="room" required>
            <select
              id="room"
              name="room"
              value={form.roomId}
              onChange={(e) => update("roomId", e.target.value)}
              required
              disabled={roomsLoading}
              className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-100"
            >
              <option value="">
                {roomsLoading ? "Lade Räume …" : "Raum auswählen …"}
              </option>
              {rooms.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.building ? `${r.building} – ${r.name}` : r.name}
                </option>
              ))}
            </select>
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Startzeit" htmlFor="start" required>
            <Input
              id="start"
              name="start"
              type="time"
              value={form.startTime}
              onChange={(e) => update("startTime", e.target.value)}
              required
            />
          </Field>
          <Field label="Endzeit" htmlFor="end" required>
            <Input
              id="end"
              name="end"
              type="time"
              value={form.endTime}
              onChange={(e) => update("endTime", e.target.value)}
              required
            />
          </Field>
        </div>

        <Field label="Notiz (optional)" htmlFor="notes">
          <textarea
            id="notes"
            name="notes"
            value={form.notes}
            onChange={(e) => update("notes", e.target.value)}
            rows={2}
            maxLength={500}
            placeholder="Zusätzliche Hinweise für das Team …"
            className="w-full resize-none rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none"
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

interface FieldProps {
  label: string;
  htmlFor: string;
  required?: boolean;
  children: React.ReactNode;
}

function Field({ label, htmlFor, required = false, children }: FieldProps) {
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
