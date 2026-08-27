"use client";

// Klassenlisten-Verwaltung (#2382): der vollständige Klassenverband aus
// Admin-Sicht. Reguläre Kinder erscheinen read-only ("In moto angelegt",
// gepflegt werden sie in der Kinder-Datenbank); Klassenlisteneinträge, also
// Kinder OHNE OGS-Datensatz mit nur Vorname, Nachname und Klasse, werden hier
// angelegt, verschoben, gelöscht und bei Dubletten bewusst einem regulären
// Kind zugeordnet. Nur so sieht die Pflegekraft pro Klasse, welche Kinder
// schon erfasst sind und welche noch fehlen.
//
// Design follows the Anmeldungen/Planung surface language: calm content
// section, uppercase kicker, gray-50 stats, no colored dashboards.

import Link from "next/link";
import { useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { Upload } from "lucide-react";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { Alert } from "~/components/ui/alert";
import { BackButton } from "~/components/ui/back-button";
import { Button } from "~/components/ui/button";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { PageIntro } from "~/components/ui/page-intro";
import { Skeleton } from "~/components/ui/skeleton";
import { formatCount } from "~/lib/format-utils";
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
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import type { Student } from "~/lib/student-helpers";
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

async function fetchClassSuggestions(): Promise<string[]> {
  const response = await fetch("/api/students/school-classes", {
    credentials: "include",
    cache: "no-store",
  });
  if (!response.ok) return [];
  const payload = (await response.json()) as { data?: string[] };
  return payload.data ?? [];
}

// Minimal projection of a regular student for the roster view: this page
// answers "wer ist schon in moto?", nicht mehr; alles Weitere gehört in die
// Kinder-Datenbank.
interface RosterStudent {
  id: string;
  firstName: string;
  lastName: string;
  schoolClass: string;
}

// The /api/students route maps the backend rows through mapStudentResponse,
// which emits the surname as `second_name` (see ~/lib/student-helpers) —
// pinning the wire type to that mapper output keeps this page honest about
// what actually arrives.
type RosterStudentWire = Pick<
  Student,
  "id" | "first_name" | "second_name" | "school_class"
>;

interface RosterStudentsPage {
  data?: {
    data?: RosterStudentWire[];
    pagination?: { total_pages?: number };
  };
}

const ROSTER_PAGE_SIZE = 1000;

// Loads EVERY student page: the /api/students route caps page_size at 1000,
// so a larger school needs more than one request or the roster (and every
// derived count) would silently stop at the first thousand children. A failed
// page THROWS — a partial roster rendered as if complete would misreport who
// is already in moto.
async function fetchRosterStudents(): Promise<RosterStudent[]> {
  const students: RosterStudent[] = [];
  let page = 1;
  let totalPages = 1;
  while (page <= totalPages) {
    const response = await fetch(
      `/api/students?page=${page}&page_size=${ROSTER_PAGE_SIZE}`,
      { credentials: "include", cache: "no-store" },
    );
    if (!response.ok) {
      throw new Error(
        `Kinderliste konnte nicht geladen werden (Seite ${page}, Status ${response.status})`,
      );
    }
    // The route wrapper wraps GET results, so the paginated list arrives as
    // { data: { data: [...], pagination } }.
    const payload = (await response.json()) as RosterStudentsPage;
    const rows = payload.data?.data ?? [];
    students.push(
      ...rows.map((student) => ({
        id: String(student.id),
        firstName: student.first_name ?? "",
        lastName: student.second_name ?? "",
        schoolClass: student.school_class ?? "",
      })),
    );
    totalPages = payload.data?.pagination?.total_pages ?? page;
    page += 1;
  }
  return students;
}

// One row of the combined roster: a regular student (read-only) or a
// class-list-only entry (editable).
type RosterRow =
  | { kind: "student"; student: RosterStudent }
  | { kind: "entry"; entry: ClassListEntry };

function rowName(row: RosterRow): { firstName: string; lastName: string } {
  return row.kind === "student" ? row.student : row.entry;
}

function rowClass(row: RosterRow): string {
  return row.kind === "student"
    ? row.student.schoolClass
    : row.entry.schoolClass;
}

// Sort key that keeps "Klasse 1a" ≡ "1a" together and orders grades
// numerically (1a before 10a) — the same equivalence the exports use.
function classSortKey(schoolClass: string): string {
  const normalized = schoolClass
    .trim()
    .toLowerCase()
    .replace(/^klasse\s+/, "");
  return normalized.replace(/^\d+/, (digits) => digits.padStart(3, "0"));
}

function rowSortKey(row: RosterRow): string {
  const { firstName, lastName } = rowName(row);
  return `${classSortKey(rowClass(row))} ${lastName.toLowerCase()} ${firstName.toLowerCase()}`;
}

export default function ClassListEntriesPage() {
  const toast = useToast();
  const { data: session } = useSession();
  // Mirror the backend route gates (api/classlistentries): create needs
  // users:create (der Sammelimport ebenso), edit users:update, delete und
  // Zuordnen users:delete. Ohne das Recht gibt es keinen Einstieg in den
  // Dialog; der Backend-403 bleibt Defense-in-Depth, keine UX.
  const canCreate = hasPermission(session, "users:create");
  const canUpdate = hasPermission(session, "users:update");
  const canDelete = hasPermission(session, "users:delete");
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
  // Die regulären Kinder gehören mit auf diese Seite: ohne sie kann niemand
  // beurteilen, welche Kinder einer Klasse schon erfasst sind und welche
  // fehlen. Nach jeder Eintrags-Änderung mit-revalidiert (Zuordnen ersetzt
  // einen Eintrag durch das reguläre Kind).
  const {
    data: students,
    error: studentsError,
    mutate: mutateStudents,
  } = useSWRAuth("class-list-roster-students", fetchRosterStudents);
  const [classFilter, setClassFilter] = useState("all");

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
  // Das Zuordnungs-Ziel: bei genau einem Kandidaten vorbelegt, bei mehreren
  // gleichnamigen Kindern MUSS die Person bewusst wählen, sonst könnte der
  // Eintrag dem falschen Kind zugeschlagen und dabei gelöscht werden.
  const [assignTarget, setAssignTarget] = useState<string | null>(null);

  const entryRows = useMemo(() => entries ?? [], [entries]);
  const duplicateCount = useMemo(
    () =>
      entryRows.filter((entry) => entry.matchingStudentIds.length > 0).length,
    [entryRows],
  );

  const allRows = useMemo<RosterRow[]>(
    () => [
      ...(students ?? []).map((student): RosterRow => ({
        kind: "student",
        student,
      })),
      ...entryRows.map((entry): RosterRow => ({ kind: "entry", entry })),
    ],
    [students, entryRows],
  );

  // Klassenoptionen aus BEIDEN Quellen, dedupliziert über dieselbe
  // Normalisierung wie die Sortierung (erste Schreibweise gewinnt).
  const classOptions = useMemo(() => {
    const seen = new Map<string, string>();
    for (const row of allRows) {
      const display = rowClass(row).trim();
      if (display === "") continue;
      const key = classSortKey(display);
      if (!seen.has(key)) seen.set(key, display);
    }
    return [...seen.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, display]) => ({ key, display }));
  }, [allRows]);

  const rows = useMemo(
    () =>
      classFilter === "all"
        ? allRows
        : allRows.filter((row) => classSortKey(rowClass(row)) === classFilter),
    [allRows, classFilter],
  );

  const classCount = classOptions.length;
  // Statuszeile des Seitenkopfs aus den bereits geladenen Zeilen.
  const statusLine = [
    `${formatCount(allRows.length)} ${allRows.length === 1 ? "Kind" : "Kinder"}`,
    `${formatCount(classCount)} ${classCount === 1 ? "Klasse" : "Klassen"}`,
    `${formatCount(duplicateCount)} mögliche ${duplicateCount === 1 ? "Dublette" : "Dubletten"}`,
  ].join(" · ");
  const studentCount = students?.length ?? 0;

  const closeModal = () => {
    setModal(null);
    setModalError(null);
    setForm(EMPTY_FORM);
    setAssignTarget(null);
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
    const studentId = assignTarget;
    if (!studentId) return;
    setSaving(true);
    try {
      await assignClassListEntry(modal.entry.id, studentId);
      toast.success("Eintrag zugeordnet und aus der Klassenliste entfernt");
      await Promise.all([mutate(), mutateStudents()]);
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

  const columns: DataTableColumn<RosterRow>[] = [
    {
      key: "name",
      header: "Name",
      render: (row) => {
        const { firstName, lastName } = rowName(row);
        return (
          <span
            className={
              row.kind === "student"
                ? "text-gray-700"
                : "font-medium text-gray-900"
            }
          >
            {lastName}, {firstName}
          </span>
        );
      },
      sortValue: (row) => {
        const { firstName, lastName } = rowName(row);
        return `${lastName.toLowerCase()} ${firstName.toLowerCase()}`;
      },
    },
    {
      key: "schoolClass",
      header: "Klasse",
      render: (row) => <span className="text-gray-700">{rowClass(row)}</span>,
      sortValue: rowSortKey,
    },
    {
      key: "status",
      header: "Status",
      render: (row) => {
        if (row.kind === "student") {
          return (
            <StatusBadge
              tone="green"
              label="In moto angelegt"
              title="Reguläres Kind mit vollständigem Datensatz. Gepflegt wird es in der Kinder-Datenbank."
            />
          );
        }
        return row.entry.matchingStudentIds.length > 0 ? (
          <StatusBadge
            tone="orange"
            label="Mögliche Dublette"
            title="Ein regulär angelegtes Kind trägt denselben Namen in derselben Klasse. Bitte prüfen und bewusst zuordnen."
          />
        ) : (
          <StatusBadge tone="gray" label="Keine Betreuung" />
        );
      },
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (row) => {
        if (row.kind === "student") {
          return (
            <div className="flex items-center justify-end">
              <Link
                href={`/students/${row.student.id}`}
                className="rounded-md px-2.5 py-1 text-sm font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900"
              >
                Öffnen
              </Link>
            </div>
          );
        }
        if (!canUpdate && !canDelete) return null;
        const entry = row.entry;
        return (
          <div className="flex items-center justify-end gap-1">
            {canDelete && entry.matchingStudentIds.length > 0 && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => {
                  setModalError(null);
                  setAssignTarget(
                    entry.matchingStudentIds.length === 1
                      ? (entry.matchingStudentIds[0] ?? null)
                      : null,
                  );
                  setModal({ kind: "assign", entry });
                }}
              >
                Zuordnen
              </Button>
            )}
            {canUpdate && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => openEdit(entry)}
              >
                Bearbeiten
              </Button>
            )}
            {canDelete && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="text-moto-red hover:text-moto-red-hover"
                onClick={() => {
                  setModalError(null);
                  setModal({ kind: "delete", entry });
                }}
              >
                Löschen
              </Button>
            )}
          </div>
        );
      },
    },
  ];

  return (
    <div className="w-full space-y-6">
      <BackButton referrer="/database/students" />

      <PageIntro
        kicker="Datenverwaltung"
        title="Klassenliste"
        description={
          isLoading && entries === undefined ? (
            <Skeleton className="h-4 w-64" />
          ) : (
            statusLine
          )
        }
        actions={
          // Der Klassenfilter steht in der Titelzeile der Karte, damit er
          // nicht als eigene, fast leere Zeile über der Tabelle liegt.
          <div className="flex flex-wrap items-center gap-2">
            <div className="w-48">
              <CustomSelect
                id="class-list-filter"
                ariaLabel="Klasse filtern"
                value={classFilter}
                options={[
                  { value: "all", label: "Alle Klassen" },
                  ...classOptions.map((option) => ({
                    value: option.key,
                    label: option.display,
                  })),
                ]}
                onChange={setClassFilter}
              />
            </div>
            {canCreate ? (
              <>
                <Link
                  href="/database/students/class-list/import"
                  className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-4 text-sm font-medium text-gray-700 hover:bg-gray-50"
                >
                  <Upload className="h-4 w-4" aria-hidden="true" />
                  Sammelimport
                </Link>
                <DatabaseCreateAction
                  label="Eintrag"
                  ariaLabel="Klassenlisteneintrag anlegen"
                  onClick={openCreate}
                />
              </>
            ) : null}
          </div>
        }
      >
        <div className="grid grid-cols-2 gap-2 sm:max-w-2xl sm:grid-cols-5">
          <div className="rounded-xl bg-gray-50 px-3 py-2">
            <span className="block text-sm font-semibold text-gray-900">
              {allRows.length}
            </span>
            <span className="block text-[11px] font-medium text-gray-500">
              Kinder gesamt
            </span>
          </div>
          <div className="rounded-xl bg-gray-50 px-3 py-2">
            <span className="block text-sm font-semibold text-gray-900">
              {studentCount}
            </span>
            <span className="block text-[11px] font-medium text-gray-500">
              In moto angelegt
            </span>
          </div>
          <div className="rounded-xl bg-gray-50 px-3 py-2">
            <span className="block text-sm font-semibold text-gray-900">
              {entryRows.length}
            </span>
            <span className="block text-[11px] font-medium text-gray-500">
              Ohne Betreuung
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

        {studentsError ? (
          <div className="mt-4">
            <Alert
              type="error"
              message="Die regulär angelegten Kinder konnten nicht vollständig geladen werden. Die Liste ist unvollständig, bitte laden Sie die Seite neu."
            />
          </div>
        ) : null}

        <div className="mt-4">
          <DataTable
            columns={columns}
            rows={rows}
            getRowKey={(row) =>
              row.kind === "student"
                ? `student-${row.student.id}`
                : `entry-${row.entry.id}`
            }
            isLoading={isLoading && entries === undefined}
            defaultSortKey="schoolClass"
            pageSize={50}
            paginationResetKey={classFilter}
            emptyState={
              <EmptyState
                title={
                  classFilter === "all"
                    ? "Noch keine Kinder erfasst"
                    : "Keine Kinder in dieser Klasse"
                }
                description="Legen Sie Kinder ohne OGS-Betreuung mit Name und Klasse an, damit Klassenlisten den vollständigen Klassenverband zeigen."
                action={
                  canCreate ? (
                    <Button
                      type="button"
                      variant="primary"
                      size="md"
                      onClick={openCreate}
                    >
                      Eintrag anlegen
                    </Button>
                  ) : undefined
                }
              />
            }
          />
        </div>
      </PageIntro>

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
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, firstName: e.target.value }))
                }
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
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, lastName: e.target.value }))
                }
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
              onChange={(e) =>
                setForm((prev) => ({ ...prev, schoolClass: e.target.value }))
              }
              placeholder="z.B. 1a"
              list="class-list-class-suggestions"
            />
            <datalist id="class-list-class-suggestions">
              {(classSuggestions ?? []).map((klass) => (
                <option key={klass} value={klass} />
              ))}
            </datalist>
            <p className="mt-1 text-xs text-gray-500">
              Genau wie bei den regulären Kindern geschrieben, damit der Eintrag
              in derselben Klassenliste landet.
            </p>
          </div>
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
        isConfirmDisabled={!assignTarget}
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
              Klassenlisteneintrag entfernt, das Kind steht dann nur noch über
              seinen regulären Datensatz auf der Klassenliste.
            </p>
            {modal.entry.matchingStudentIds.length > 1 ? (
              <fieldset className="space-y-2">
                <legend className="text-sm font-medium text-gray-900">
                  Mehrere Kinder tragen diesen Namen in dieser Klasse. Bitte den
                  richtigen Datensatz auswählen:
                </legend>
                {modal.entry.matchingStudentIds.map((studentId) => (
                  <label
                    key={studentId}
                    className="flex items-center justify-between gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700"
                  >
                    <span className="flex items-center gap-2">
                      <input
                        type="radio"
                        name="assign-target"
                        className="accent-moto-green"
                        checked={assignTarget === studentId}
                        onChange={() => setAssignTarget(studentId)}
                      />
                      Kind-Datensatz #{studentId}
                    </span>
                    <Link
                      href={`/students/${studentId}`}
                      target="_blank"
                      className="text-xs font-medium text-gray-600 underline hover:text-gray-900"
                    >
                      Öffnen
                    </Link>
                  </label>
                ))}
              </fieldset>
            ) : null}
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
