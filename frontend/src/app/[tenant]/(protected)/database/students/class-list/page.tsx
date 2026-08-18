"use client";

// Klassenlisteneinträge (#2382): Kinder des Klassenverbands OHNE
// OGS-Datensatz — nur Vorname, Nachname, Klasse. Sie erscheinen auf
// Klassenlisten und in der Klassenansicht ("Keine Betreuung"), nie in
// Kindersuche, Anwesenheit, Betreuungsplanung oder Elternportal. Diese Seite
// ist die Verwaltung: anlegen, in andere Klasse verschieben, löschen und
// Dubletten bewusst einem regulären Kind zuordnen.
//
// Design follows the Anmeldungen/Planung surface language: calm content
// section, uppercase kicker, gray-50 stats, no colored dashboards.

import Link from "next/link";
import { useMemo, useState } from "react";
import { Upload } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { BackButton } from "~/components/ui/back-button";
import { Button } from "~/components/ui/button";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { StatusBadge } from "~/components/ui/status-badge";
import { useToast } from "~/contexts/ToastContext";
import {
  assignClassListEntry,
  createClassListEntry,
  deleteClassListEntry,
  fetchClassListEntries,
  updateClassListEntry,
  type ClassListEntry,
} from "~/lib/class-list-entries-api";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "ClassListEntriesPage" });

const SWR_KEY = "class-list-entries";

interface EntryFormState {
  firstName: string;
  lastName: string;
  schoolClass: string;
}

const EMPTY_FORM: EntryFormState = {
  firstName: "",
  lastName: "",
  schoolClass: "",
};

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : "Unbekannter Fehler";
}

function EntryFormFields({
  form,
  onChange,
  classSuggestions,
}: Readonly<{
  form: EntryFormState;
  onChange: (next: EntryFormState) => void;
  classSuggestions: string[];
}>) {
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <div>
          <label
            htmlFor="entry-first-name"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Vorname
          </label>
          <Input
            id="entry-first-name"
            value={form.firstName}
            onChange={(e) => onChange({ ...form, firstName: e.target.value })}
            placeholder="z.B. Lena"
          />
        </div>
        <div>
          <label
            htmlFor="entry-last-name"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Nachname
          </label>
          <Input
            id="entry-last-name"
            value={form.lastName}
            onChange={(e) => onChange({ ...form, lastName: e.target.value })}
            placeholder="z.B. Beispiel"
          />
        </div>
      </div>
      <div>
        <label
          htmlFor="entry-school-class"
          className="mb-1 block text-sm font-medium text-gray-700"
        >
          Klasse
        </label>
        <Input
          id="entry-school-class"
          value={form.schoolClass}
          onChange={(e) => onChange({ ...form, schoolClass: e.target.value })}
          placeholder="z.B. 1a"
          list="class-list-class-suggestions"
        />
        <datalist id="class-list-class-suggestions">
          {classSuggestions.map((klass) => (
            <option key={klass} value={klass} />
          ))}
        </datalist>
        <p className="mt-1 text-xs text-gray-500">
          Genau wie bei den regulären Kindern geschrieben, damit der Eintrag in
          derselben Klassenliste landet.
        </p>
      </div>
    </div>
  );
}

async function fetchClassSuggestions(): Promise<string[]> {
  const response = await fetch("/api/students/school-classes", {
    credentials: "include",
    cache: "no-store",
  });
  if (!response.ok) return [];
  const payload = (await response.json()) as { data?: string[] };
  return payload.data ?? [];
}

export default function ClassListEntriesPage() {
  const toast = useToast();
  const {
    data: entries,
    error: loadError,
    isLoading,
    mutate,
  } = useSWRAuth(SWR_KEY, fetchClassListEntries);
  const { data: classSuggestions } = useSWRAuth(
    "class-list-class-suggestions",
    fetchClassSuggestions,
  );

  const [modal, setModal] = useState<
    | { kind: "create" }
    | { kind: "edit"; entry: ClassListEntry }
    | { kind: "delete"; entry: ClassListEntry }
    | { kind: "assign"; entry: ClassListEntry }
    | null
  >(null);
  const [form, setForm] = useState<EntryFormState>(EMPTY_FORM);
  const [modalError, setModalError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const rows = useMemo(() => entries ?? [], [entries]);
  const duplicateCount = useMemo(
    () => rows.filter((entry) => entry.matchingStudentIds.length > 0).length,
    [rows],
  );
  const classCount = useMemo(
    () =>
      new Set(rows.map((entry) => entry.schoolClass.toLowerCase().trim())).size,
    [rows],
  );

  const closeModal = () => {
    setModal(null);
    setModalError(null);
    setForm(EMPTY_FORM);
  };

  const openCreate = () => {
    setForm(EMPTY_FORM);
    setModalError(null);
    setModal({ kind: "create" });
  };

  const openEdit = (entry: ClassListEntry) => {
    setForm({
      firstName: entry.firstName,
      lastName: entry.lastName,
      schoolClass: entry.schoolClass,
    });
    setModalError(null);
    setModal({ kind: "edit", entry });
  };

  const submitForm = async () => {
    if (modal?.kind !== "create" && modal?.kind !== "edit") return;
    setSaving(true);
    setModalError(null);
    try {
      if (modal.kind === "create") {
        await createClassListEntry(form);
        toast.success("Eintrag angelegt");
      } else {
        await updateClassListEntry(modal.entry.id, form);
        toast.success("Eintrag gespeichert");
      }
      await mutate();
      closeModal();
    } catch (err) {
      logger.error("class_list_entry_save_failed", {
        error: errorMessage(err),
      });
      setModalError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (modal?.kind !== "delete") return;
    setSaving(true);
    try {
      await deleteClassListEntry(modal.entry.id);
      toast.success("Eintrag gelöscht");
      await mutate();
      closeModal();
    } catch (err) {
      logger.error("class_list_entry_delete_failed", {
        error: errorMessage(err),
      });
      setModalError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const confirmAssign = async () => {
    if (modal?.kind !== "assign") return;
    const studentId = modal.entry.matchingStudentIds[0];
    if (!studentId) return;
    setSaving(true);
    try {
      await assignClassListEntry(modal.entry.id, studentId);
      toast.success("Eintrag zugeordnet und aus der Klassenliste entfernt");
      await mutate();
      closeModal();
    } catch (err) {
      logger.error("class_list_entry_assign_failed", {
        error: errorMessage(err),
      });
      setModalError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const columns: DataTableColumn<ClassListEntry>[] = [
    {
      key: "name",
      header: "Name",
      render: (entry) => (
        <span className="font-medium text-gray-900">
          {entry.lastName}, {entry.firstName}
        </span>
      ),
      sortValue: (entry) =>
        `${entry.lastName.toLowerCase()} ${entry.firstName.toLowerCase()}`,
    },
    {
      key: "schoolClass",
      header: "Klasse",
      render: (entry) => (
        <span className="text-gray-700">{entry.schoolClass}</span>
      ),
      sortValue: (entry) => entry.schoolClass.toLowerCase(),
    },
    {
      key: "status",
      header: "Status",
      render: (entry) =>
        entry.matchingStudentIds.length > 0 ? (
          <StatusBadge
            tone="orange"
            label="Mögliche Dublette"
            title="Ein regulär angelegtes Kind trägt denselben Namen in derselben Klasse. Bitte prüfen und bewusst zuordnen."
          />
        ) : (
          <StatusBadge tone="gray" label="Keine Betreuung" />
        ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (entry) => (
        <div className="flex items-center justify-end gap-1">
          {entry.matchingStudentIds.length > 0 && (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => {
                setModalError(null);
                setModal({ kind: "assign", entry });
              }}
            >
              Zuordnen
            </Button>
          )}
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => openEdit(entry)}
          >
            Bearbeiten
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="compact"
            className="text-[#DC2626] hover:text-[#DC2626]"
            onClick={() => {
              setModalError(null);
              setModal({ kind: "delete", entry });
            }}
          >
            Löschen
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className="w-full space-y-4">
      <BackButton referrer="/database/students" />

      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Klassenlisteneinträge
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Kinder ohne OGS-Betreuung
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Diese Einträge vervollständigen den Klassenverband auf
              Klassenlisten und in der Klassenansicht. Sie bestehen nur aus Name
              und Klasse — ohne Betreuung, Anwesenheit oder Kontaktdaten.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Link
              href="/database/students/class-list/import"
              className="flex h-9 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              <Upload className="h-4 w-4" aria-hidden="true" />
              Sammelimport
            </Link>
            <Button
              type="button"
              variant="primary"
              size="md"
              className="h-9"
              onClick={openCreate}
            >
              Eintrag anlegen
            </Button>
          </div>
        </div>

        <div className="mt-4 grid grid-cols-2 gap-2 sm:max-w-md sm:grid-cols-3">
          <div className="rounded-xl bg-gray-50 px-3 py-2">
            <span className="block text-sm font-semibold text-gray-900">
              {rows.length}
            </span>
            <span className="block text-[11px] font-medium text-gray-500">
              Einträge
            </span>
          </div>
          <div className="rounded-xl bg-gray-50 px-3 py-2">
            <span className="block text-sm font-semibold text-gray-900">
              {classCount}
            </span>
            <span className="block text-[11px] font-medium text-gray-500">
              Klassen
            </span>
          </div>
          <div className="rounded-xl bg-gray-50 px-3 py-2">
            <span className="block text-sm font-semibold text-gray-900">
              {duplicateCount}
            </span>
            <span className="block text-[11px] font-medium text-gray-500">
              Mögliche Dubletten
            </span>
          </div>
        </div>

        {loadError ? (
          <div className="mt-4">
            <Alert
              type="error"
              message="Die Klassenlisteneinträge konnten nicht geladen werden."
            />
          </div>
        ) : null}

        <div className="mt-4">
          <DataTable
            columns={columns}
            rows={rows}
            getRowKey={(entry) => entry.id}
            isLoading={isLoading && entries === undefined}
            defaultSortKey="schoolClass"
            emptyState={
              <EmptyState
                title="Noch keine Klassenlisteneinträge"
                description="Legen Sie Kinder ohne OGS-Betreuung mit Name und Klasse an, damit Klassenlisten den vollständigen Klassenverband zeigen."
                action={
                  <Button
                    type="button"
                    variant="primary"
                    size="md"
                    onClick={openCreate}
                  >
                    Eintrag anlegen
                  </Button>
                }
              />
            }
          />
        </div>
      </section>

      <Modal
        isOpen={modal?.kind === "create" || modal?.kind === "edit"}
        onClose={closeModal}
        title={
          modal?.kind === "edit"
            ? "Eintrag bearbeiten"
            : "Klassenlisteneintrag anlegen"
        }
      >
        <div className="space-y-4">
          {modalError ? <Alert type="error" message={modalError} /> : null}
          <EntryFormFields
            form={form}
            onChange={setForm}
            classSuggestions={classSuggestions ?? []}
          />
          <div className="flex justify-end gap-2 border-t border-gray-100 pt-4">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={closeModal}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              disabled={
                saving ||
                !form.firstName.trim() ||
                !form.lastName.trim() ||
                !form.schoolClass.trim()
              }
              onClick={() => void submitForm()}
            >
              {saving
                ? "Speichern…"
                : modal?.kind === "edit"
                  ? "Speichern"
                  : "Anlegen"}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmationModal
        isOpen={modal?.kind === "delete"}
        onClose={closeModal}
        onConfirm={() => void confirmDelete()}
        title="Eintrag löschen"
        confirmText="Löschen"
        isConfirmLoading={saving}
      >
        {modal?.kind === "delete" ? (
          <div className="space-y-3">
            {modalError ? <Alert type="error" message={modalError} /> : null}
            <p className="text-sm text-gray-600">
              Soll der Eintrag{" "}
              <span className="font-medium text-gray-900">
                {modal.entry.firstName} {modal.entry.lastName} (
                {modal.entry.schoolClass})
              </span>{" "}
              gelöscht werden? Er verschwindet damit von allen Klassenlisten.
            </p>
          </div>
        ) : null}
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={modal?.kind === "assign"}
        onClose={closeModal}
        onConfirm={() => void confirmAssign()}
        title="Eintrag zuordnen"
        confirmText="Zuordnen"
        isConfirmLoading={saving}
      >
        {modal?.kind === "assign" ? (
          <div className="space-y-3">
            {modalError ? <Alert type="error" message={modalError} /> : null}
            <p className="text-sm text-gray-600">
              <span className="font-medium text-gray-900">
                {modal.entry.firstName} {modal.entry.lastName} (
                {modal.entry.schoolClass})
              </span>{" "}
              ist inzwischen als reguläres Kind angelegt. Beim Zuordnen wird der
              Klassenlisteneintrag entfernt — das Kind steht dann nur noch über
              seinen regulären Datensatz auf der Klassenliste.
            </p>
            <p className="text-xs text-gray-500">
              Die Zuordnung passiert nie automatisch: Bitte nur bestätigen, wenn
              es sich wirklich um dasselbe Kind handelt.
            </p>
          </div>
        ) : null}
      </ConfirmationModal>
    </div>
  );
}
