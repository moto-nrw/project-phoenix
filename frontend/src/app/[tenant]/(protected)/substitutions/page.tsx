"use client";

import { Clock } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useSession } from "next-auth/react";
import { type ReactNode, Suspense, useMemo, useState } from "react";

import { RoleGuard } from "~/components/auth/role-guard";
import { Alert } from "~/components/ui/alert";
import { Avatar } from "~/components/ui/avatar";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useToast } from "~/contexts/ToastContext";
import { groupService } from "~/lib/api";
import type { Group } from "~/lib/api";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { substitutionService } from "~/lib/substitution-api";
import type {
  Substitution,
  TeacherAvailability,
} from "~/lib/substitution-helpers";
import { formatTeacherName } from "~/lib/substitution-helpers";
import { useImmutableSWR, useSWRAuth } from "~/lib/swr";
import { useOpenCareGroupMode } from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "SubstitutionsPage" });

/** Wert des Dauer-Umschalters, der das freie Zahlenfeld einblendet. */
const CUSTOM_DURATION = "custom";

/**
 * Voreinstellungen des Dauer-Umschalters. Die Werte sind Tageszahlen, weil der
 * erste Tag mitzählt: "Heute" ist ein Tag, nicht null.
 */
const DURATION_PRESETS = [
  { value: "1", label: "1 Tag" },
  { value: "3", label: "3 Tage" },
  { value: "7", label: "1 Woche" },
  { value: CUSTOM_DURATION, label: "Individuell" },
];

const MAX_DURATION_DAYS = 365;

/**
 * Enddatum einer Übergabe. Der Starttag zählt mit.
 */
function accessEndDate(startDate: string, days: number): string {
  const end = parseISODate(startDate);
  end.setDate(end.getDate() + days - 1);
  return toISODate(end);
}

function getSubstituteName(
  teachers: TeacherAvailability[],
  substitution: Substitution,
): string {
  const substituteTeacher = teachers.find(
    (t) => t.id === substitution.substituteStaffId,
  );
  return substituteTeacher
    ? formatTeacherName(substituteTeacher)
    : (substitution.substituteStaffName ?? "Unbekannt");
}

/**
 * Meta-Zeile unter einem Namen oder Gruppennamen: durch Mittelpunkte getrennte
 * Textstücke, leere Werte fallen weg.
 *
 * Bewusst ohne farbige Pillen. Eine getönte Pille ist im Produkt einem echten
 * Ausnahmezustand vorbehalten (etwa "Abwesend" in der Mitarbeiterliste); hier
 * hätte fast jede Zeile eine getragen, die meisten davon für den Normalfall
 * "Verfügbar". Farbe, die auf jeder Zeile steht, unterscheidet nichts mehr und
 * macht aus einer Arbeitsliste ein Schaubild.
 */
function MetaLine({ parts }: Readonly<{ parts: (string | null)[] }>) {
  const visible = parts.filter((part): part is string => Boolean(part));
  if (visible.length === 0) return null;

  return (
    <p className="mt-1 truncate text-xs text-gray-500">
      {visible.map((part, index) => (
        <span key={part}>
          {index > 0 && <span className="mx-1.5 text-gray-300">·</span>}
          {part}
        </span>
      ))}
    </p>
  );
}

/**
 * Gemeinsame Listenfläche INNERHALB einer Karte: Trennlinien statt einzelner
 * Karten. Die Karte darum bringt die Abschnittskarte mit, damit kein Titel
 * frei auf dem Grund steht.
 */
function ListSurface({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <ul className="divide-y divide-gray-200 overflow-hidden rounded-xl border border-gray-200">
      {children}
    </ul>
  );
}

/** Zahl der Einträge, rechts im Kopf der Abschnittskarte. */
function SectionCount({ count }: Readonly<{ count: number }>) {
  return (
    <span className="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-gray-100 px-2 text-xs font-semibold text-gray-700 tabular-nums">
      {count}
    </span>
  );
}

/** Eine Zeile in der Liste der Gruppenübergaben. */
function AccessRow({
  groupName,
  personName,
  until,
  onEnd,
  disabled,
}: Readonly<{
  groupName: string;
  personName: string;
  until: string | null;
  onEnd: () => void;
  disabled: boolean;
}>) {
  return (
    <li className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">
          {groupName}
        </p>
        <MetaLine parts={[`Übergeben an ${personName}`, until]} />
      </div>
      {/* Zeilenaktionen stehen im Kebab der Zeile (Bauart 1 Regel 4), nicht
          als eigene Schaltfläche neben jedem Eintrag. */}
      <OverflowMenu
        ariaLabel={`Aktionen für ${groupName} und ${personName}`}
        items={[
          {
            label: "Übergabe beenden",
            destructive: true,
            disabled,
            onClick: onEnd,
          },
        ]}
      />
    </li>
  );
}

function SubstitutionPageContent() {
  const router = useTenantRouter();
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  // Gruppenübergaben sind nur bei festen Gruppen sinnvoll (#1940); bei offener
  // Betreuung arbeiten ohnehin alle Berechtigten mit allen Kindern. Die
  // Navigation blendet den Eintrag aus, das hier fängt Direktaufrufe ab.
  const openCareGroupMode = useOpenCareGroupMode();

  // Der Seitenkopf trägt den Titel nur im Telefonformat; auf breiten Schirmen
  // steht er bereits in der Kopfleiste der Anwendung.

  const { success: showSuccessToast } = useToast();

  const {
    data: teachers = [],
    isLoading: teachersLoading,
    error: teachersError,
    mutate: mutateTeachers,
  } = useSWRAuth<TeacherAvailability[]>(
    "substitution-teachers",
    () => substitutionService.fetchAvailableTeachers(),
    { keepPreviousData: true },
  );

  const {
    data: groups = [],
    isLoading: groupsLoading,
    error: groupsError,
  } = useImmutableSWR<Group[]>("substitution-groups", () =>
    groupService.getGroups(),
  );

  const {
    data: activeSubstitutions = [],
    isLoading: handoversLoading,
    error: handoversError,
    mutate: mutateActiveSubstitutions,
  } = useSWRAuth<Substitution[]>(
    "group-handovers",
    () => substitutionService.fetchSubstitutions(),
    { keepPreviousData: true },
  );

  // UI-Zustand
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [isMutating, setIsMutating] = useState(false);

  // Ladefehler gehören auf die Seite, Fehler einer Aktion in den Dialog, in dem
  // die Aktion ausgelöst wurde. Vorher war das eine gemeinsame Variable, wodurch
  // ein Ladefehler mitten im Zuweisen-Dialog auftauchte.
  const [mutationError, setMutationError] = useState<string | null>(null);

  // Zuweisen-Dialog
  const [showPopup, setShowPopup] = useState(false);
  const [selectedTeacher, setSelectedTeacher] =
    useState<TeacherAvailability | null>(null);
  const [selectedGroupId, setSelectedGroupId] = useState("");
  const [startDate, setStartDate] = useState(() => berlinTodayISO());
  const [durationPreset, setDurationPreset] = useState("1");
  const [customDays, setCustomDays] = useState(2);

  // Beenden-Bestätigung
  const [showEndConfirmation, setShowEndConfirmation] = useState(false);
  const [substitutionToEnd, setSubstitutionToEnd] = useState<{
    id: string;
    groupName: string;
    teacherName: string;
  } | null>(null);

  const isLoading = teachersLoading || groupsLoading || handoversLoading;
  const loadError =
    teachersError || groupsError || handoversError
      ? "Die Daten konnten nicht geladen werden. Bitte laden Sie die Seite neu."
      : null;

  const substitutionDays =
    durationPreset === CUSTOM_DURATION ? customDays : Number(durationPreset);

  const filteredTeachers = useMemo(() => {
    const assignmentCounts = new Map<string, number>();
    for (const handover of activeSubstitutions) {
      assignmentCounts.set(
        handover.substituteStaffId,
        (assignmentCounts.get(handover.substituteStaffId) ?? 0) + 1,
      );
    }
    let filtered = teachers.map((teacher) => {
      const count = assignmentCounts.get(teacher.id) ?? 0;
      return {
        ...teacher,
        inSubstitution: count > 0,
        substitutionCount: count,
      };
    });

    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter((teacher) => {
        const checks = [
          formatTeacherName(teacher).toLowerCase().includes(searchLower),
          teacher.role?.toLowerCase().includes(searchLower),
          teacher.regularGroup?.toLowerCase().includes(searchLower),
        ];
        return checks.some(Boolean);
      });
    }

    if (statusFilter !== "all") {
      const isInSubstitution = statusFilter === "substitution";
      filtered = filtered.filter(
        (teacher) => teacher.inSubstitution === isInSubstitution,
      );
    }

    return filtered;
  }, [activeSubstitutions, teachers, searchTerm, statusFilter]);

  const openSubstitutionPopup = (teacher: TeacherAvailability) => {
    setSelectedTeacher(teacher);
    setSelectedGroupId("");
    setStartDate(berlinTodayISO());
    setDurationPreset("1");
    setCustomDays(2);
    setMutationError(null);
    setShowPopup(true);
  };

  const closePopup = () => {
    setShowPopup(false);
    setSelectedTeacher(null);
    setMutationError(null);
  };

  const handleAssignSubstitution = async () => {
    if (!selectedTeacher || !selectedGroupId || !startDate) {
      setMutationError("Bitte wählen Sie eine Gruppe und ein Startdatum aus.");
      return;
    }

    const group = groups.find((g) => g.id === selectedGroupId);
    if (!group) {
      setMutationError("Gruppe nicht gefunden.");
      return;
    }

    try {
      setIsMutating(true);
      setMutationError(null);

      await substitutionService.createSubstitution(
        group.id,
        selectedTeacher.id,
        startDate,
        accessEndDate(startDate, substitutionDays),
      );

      await Promise.all([mutateTeachers(), mutateActiveSubstitutions()]);

      const teacherName = formatTeacherName(selectedTeacher);
      const days = substitutionDays > 1 ? `${substitutionDays} Tage` : "1 Tag";
      showSuccessToast(
        `Gruppe „${group.name}“ an ${teacherName} übergeben (${days})`,
      );

      closePopup();
    } catch (err) {
      logger.error("failed to create substitution", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMutationError("Die Gruppe konnte nicht übergeben werden.");
    } finally {
      setIsMutating(false);
    }
  };

  const handleEndSubstitutionClick = (
    substitutionId: string,
    groupName: string,
    teacherName: string,
  ) => {
    setMutationError(null);
    setSubstitutionToEnd({ id: substitutionId, groupName, teacherName });
    setShowEndConfirmation(true);
  };

  const confirmEndSubstitution = async () => {
    if (!substitutionToEnd) return;

    try {
      setIsMutating(true);
      setMutationError(null);
      await substitutionService.deleteSubstitution(substitutionToEnd.id);

      await Promise.all([mutateTeachers(), mutateActiveSubstitutions()]);

      showSuccessToast(`Übergabe von „${substitutionToEnd.groupName}“ beendet`);

      setShowEndConfirmation(false);
      setSubstitutionToEnd(null);
    } catch (err) {
      logger.error("failed to end substitution", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMutationError("Die Übergabe konnte nicht beendet werden.");
    } finally {
      setIsMutating(false);
    }
  };

  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "status",
        label: "Status",
        type: "buttons",
        value: statusFilter,
        onChange: (value) => setStatusFilter(value as string),
        options: [
          { value: "all", label: "Alle" },
          { value: "available", label: "Verfügbar" },
          { value: "substitution", label: "Hat eine Übergabe" },
        ],
      },
    ],
    [statusFilter],
  );

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (statusFilter !== "all") {
      const statusLabels = {
        available: "Verfügbar",
        substitution: "Hat eine Übergabe",
      };
      filters.push({
        id: "status",
        label:
          statusLabels[statusFilter as keyof typeof statusLabels] ??
          statusFilter,
        onRemove: () => setStatusFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, statusFilter]);

  if (status === "loading") {
    return (
      <TenantPage
        title="Gruppenübergaben"
        loadingLabel="Gruppenübergaben werden geladen…"
        loading
        testId="loading"
      />
    );
  }

  if (openCareGroupMode) {
    return (
      <TenantPage
        title="Gruppenübergaben"
        empty={{
          icon: <MotoConceptIcon concept="staff" size={48} />,
          title: "Gruppenübergaben nicht verfügbar",
          description:
            "Diese Schule arbeitet mit offener Betreuung ohne feste Gruppen. Alle berechtigten Mitarbeitenden arbeiten mit allen Kindern. Deshalb sind Gruppenübergaben nicht nötig. Die Einstellung „Arbeit mit festen Gruppen“ kann in den Einstellungen geändert werden.",
        }}
      />
    );
  }

  const renderTeacherList = () => {
    if (filteredTeachers.length === 0) {
      return (
        <EmptyState
          icon={<MotoConceptIcon concept="staff" size={48} />}
          title="Keine Fachkräfte gefunden"
          description="Passen Sie die Suche oder die Filter an."
        />
      );
    }

    return (
      <ListSurface>
        {filteredTeachers.map((teacher) => {
          const name = formatTeacherName(teacher);
          return (
            <li
              key={teacher.id}
              className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="flex min-w-0 items-center gap-3">
                <Avatar name={name} size="md" />
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-gray-900">
                    {name}
                  </p>
                  {/* "Verfügbar" ist der Normalfall und bekommt keine eigene
                      Auszeichnung: wo nichts steht, ist nichts zugewiesen. */}
                  <MetaLine
                    parts={[
                      teacher.regularGroup ?? null,
                      teacher.substitutionCount > 0
                        ? `${teacher.substitutionCount} ${teacher.substitutionCount === 1 ? "Gruppenübergabe" : "Gruppenübergaben"}`
                        : null,
                    ]}
                  />
                </div>
              </div>
              <OverflowMenu
                ariaLabel={`Aktionen für ${name}`}
                items={[
                  {
                    label: "Gruppe übergeben",
                    onClick: () => openSubstitutionPopup(teacher),
                  },
                ]}
              />
            </li>
          );
        })}
      </ListSurface>
    );
  };

  const renderAccessSection = (
    substitutions: Substitution[],
    emptyText: string,
    untilFor: (substitution: Substitution) => string | null,
  ) => {
    // Eine leere Liste bekommt dieselbe Fläche wie eine gefüllte, damit der
    // Abschnitt nicht als freischwebender Text im Raster steht. Bewusst knapp:
    // das ist eine Liste ohne Einträge, kein leerer Seitenbereich, für den
    // EmptyState mit seiner grossen zentrierten Überschrift gedacht ist.
    if (substitutions.length === 0) {
      return <p className="text-sm text-gray-500">{emptyText}</p>;
    }

    return (
      <ListSurface>
        {substitutions.map((substitution) => {
          const group = groups.find((g) => g.id === substitution.groupId);
          if (!group) return null;

          const substituteName = getSubstituteName(teachers, substitution);

          return (
            <AccessRow
              key={substitution.id}
              groupName={group.name}
              personName={substituteName}
              until={untilFor(substitution)}
              disabled={isMutating}
              onEnd={() =>
                handleEndSubstitutionClick(
                  substitution.id,
                  group.name,
                  substituteName,
                )
              }
            />
          );
        })}
      </ListSurface>
    );
  };

  const assignFooter = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={closePopup}
        disabled={isMutating}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => void handleAssignSubstitution()}
        isLoading={isMutating}
        loadingText="Wird zugewiesen…"
        disabled={!selectedGroupId || isMutating}
      >
        Zuweisen
      </Button>
    </>
  );

  // Die beiden Übergabearten, die die Seite darunter auflistet. Seit dem
  // frei wählbaren Startdatum (#2095) trennt die Tageslänge, nicht mehr ein
  // isTransfer-Flag: eintägig heißt Start- und Endtag sind derselbe Tag.
  const transfers = activeSubstitutions.filter(
    (s) => s.startDate === s.endDate,
  );
  const longTermAccess = activeSubstitutions.filter(
    (s) => s.startDate !== s.endDate,
  );

  // Statuszeile: die beiden Übergabearten. Die Zahl der Fachkräfte steht als
  // Zähler im Seitenkopf, weil sie sich mit Suche und Filter ändert.
  const statusLine = `${transfers.length} ${
    transfers.length === 1 ? "eintägige Übergabe" : "eintägige Übergaben"
  } · ${longTermAccess.length} ${
    longTermAccess.length === 1 ? "mehrtägige Übergabe" : "mehrtägige Übergaben"
  }`;

  return (
    <TenantPage
      title="Gruppenübergaben"
      loadingLabel="Gruppenübergaben werden geladen…"
      stats={statusLine}
      statsLoading={isLoading}
      badge={{
        icon: <MotoConceptIcon concept="staff" size={20} />,
        count: filteredTeachers.length,
        label: "Fachkräfte",
      }}
      search={{
        value: searchTerm,
        onChange: setSearchTerm,
        placeholder: "Fachkraft suchen…",
      }}
      filters={filterConfigs}
      activeFilters={activeFilters}
      onClearAllFilters={() => {
        setSearchTerm("");
        setStatusFilter("all");
      }}
      error={loadError}
      loading={isLoading}
      empty={
        // Leerzustand als nächster Schritt (Bauart 1 Regel 8): ohne
        // Mitarbeitende gibt es nichts zuzuweisen, also führt er dorthin.
        !isLoading && !loadError && teachers.length === 0
          ? {
              icon: <MotoConceptIcon concept="staff" size={48} />,
              title: "Noch keine Fachkräfte",
              description:
                "Legen Sie Mitarbeitende an. Danach können Sie ihnen hier eine Gruppe übergeben.",
              action: (
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={() => router.push("/staff")}
                >
                  Zu den Mitarbeitenden
                </Button>
              ),
            }
          : null
      }
      overlays={
        <>
          <Modal
            isOpen={showPopup}
            onClose={closePopup}
            title="Gruppe übergeben"
            footer={assignFooter}
            isDismissDisabled={isMutating}
          >
            <div className="space-y-6">
              {mutationError ? (
                <Alert type="error" message={mutationError} />
              ) : null}

              <p className="text-sm text-gray-600">
                <span className="font-medium text-gray-900">
                  {selectedTeacher ? formatTeacherName(selectedTeacher) : ""}
                </span>{" "}
                erhält zusätzlichen Zugriff auf die Kinder der gewählten Gruppe.
                Die bestehenden Berechtigungen der Gruppe bleiben unverändert.
              </p>

              <p className="text-sm text-gray-600">
                <span className="font-medium text-gray-900">
                  {selectedTeacher ? formatTeacherName(selectedTeacher) : ""}
                </span>{" "}
                übernimmt die Verantwortung für die gewählte Gruppe. Die Gruppe
                erscheint für diese Person unter „Meine Gruppen“. Die
                Berechtigung für Kinderdaten ändert sich nicht.
              </p>

              <div>
                <label
                  htmlFor="substitution-start-date"
                  className="mb-2 block text-sm font-medium text-gray-700"
                >
                  Startdatum
                </label>
                <Input
                  id="substitution-start-date"
                  name="substitution-start-date"
                  type="date"
                  min={berlinTodayISO()}
                  required
                  value={startDate}
                  onChange={(event) => setStartDate(event.target.value)}
                />
              </div>

              <div>
                <label
                  id="substitution-group-select-label"
                  htmlFor="substitution-group-select"
                  className="mb-2 block text-sm font-medium text-gray-700"
                >
                  OGS-Gruppe auswählen
                </label>
                <CustomSelect
                  id="substitution-group-select"
                  ariaLabelledBy="substitution-group-select-label"
                  value={selectedGroupId}
                  onChange={setSelectedGroupId}
                  placeholder="Gruppe auswählen…"
                  options={[
                    { value: "", label: "Gruppe auswählen…" },
                    ...groups.map((group) => ({
                      value: group.id,
                      label: group.name,
                    })),
                  ]}
                />
              </div>

              <div>
                <p
                  id="substitution-duration-label"
                  className="mb-2 block text-sm font-medium text-gray-700"
                >
                  Dauer
                </p>
                <SegmentedControl
                  ariaLabel="Dauer"
                  value={durationPreset}
                  onChange={setDurationPreset}
                  items={DURATION_PRESETS.map((preset) => ({
                    value: preset.value,
                    label: preset.label,
                  }))}
                />

                {durationPreset === CUSTOM_DURATION ? (
                  <div className="mt-3 w-40">
                    <Input
                      id="substitution-days-input"
                      name="substitution-days-input"
                      type="number"
                      min={1}
                      max={MAX_DURATION_DAYS}
                      controlSize="compact"
                      aria-label="Anzahl der Tage"
                      value={customDays}
                      onChange={(e) => {
                        const parsed = Number(e.target.value);
                        if (!Number.isInteger(parsed)) return;

                        setCustomDays(
                          Math.min(MAX_DURATION_DAYS, Math.max(1, parsed)),
                        );
                      }}
                    />
                  </div>
                ) : null}

                <p className="mt-2 text-sm text-gray-500">
                  {substitutionDays === 1
                    ? `Die Übergabe gilt am ${formatDate(startDate, true)}.`
                    : `Die Übergabe gilt bis ${formatDate(accessEndDate(startDate, substitutionDays), true)}.`}
                </p>
              </div>
            </div>
          </Modal>

          <ConfirmationModal
            isOpen={showEndConfirmation}
            onClose={() => {
              setShowEndConfirmation(false);
              setSubstitutionToEnd(null);
              setMutationError(null);
            }}
            onConfirm={() => void confirmEndSubstitution()}
            title="Übergabe beenden?"
            confirmText="Beenden"
            cancelText="Abbrechen"
            isConfirmLoading={isMutating}
            confirmButtonClass="bg-moto-red hover:bg-moto-red/90"
          >
            {substitutionToEnd && (
              <div className="space-y-4">
                {mutationError ? (
                  <Alert type="error" message={mutationError} />
                ) : null}
                <p className="text-sm text-gray-600">
                  Möchten Sie diese Übergabe wirklich beenden? Die Verantwortung
                  für die Gruppe liegt danach nicht mehr bei dieser Person.
                </p>
                <div className="rounded-2xl border border-gray-200 bg-gray-50 p-4">
                  <p className="text-sm text-gray-600">
                    <span className="font-medium text-gray-900">Gruppe:</span>{" "}
                    {substitutionToEnd.groupName}
                  </p>
                  <p className="mt-1 text-sm text-gray-600">
                    <span className="font-medium text-gray-900">
                      Übergeben an:
                    </span>{" "}
                    {substitutionToEnd.teacherName}
                  </p>
                </div>
              </div>
            )}
          </ConfirmationModal>
        </>
      }
    >
      {/* Jeder Abschnitt sitzt auf einer eigenen Karte: der Titel steht
          damit auf einer Fläche und nicht frei auf dem Grund. Die
          Trefferzahl der Fachkräfte steht bereits als Zähler im Seitenkopf. */}
      <SectionCard title="Verfügbare pädagogische Fachkräfte">
        {renderTeacherList()}
      </SectionCard>

      <SectionCard
        title="Eintägige Übergaben"
        description="Gelten für genau einen Tag."
        icon={Clock}
        action={<SectionCount count={transfers.length} />}
      >
        {renderAccessSection(
          transfers,
          "Keine eintägigen Übergaben",
          (substitution) => `am ${formatDate(substitution.endDate, true)}`,
        )}
      </SectionCard>

      <SectionCard
        title="Mehrtägige Übergaben"
        description="Gelten über mehrere Tage."
        leading={<MotoConceptIcon concept="calendar" size={20} />}
        action={<SectionCount count={longTermAccess.length} />}
      >
        {renderAccessSection(
          longTermAccess,
          "Keine mehrtägigen Übergaben",
          (substitution) => `bis ${formatDate(substitution.endDate, true)}`,
        )}
      </SectionCard>
    </TenantPage>
  );
}

export default function SubstitutionPage() {
  return (
    <RoleGuard variant="adminOnly">
      <Suspense
        fallback={
          <TenantPage
            title="Gruppenübergaben"
            loadingLabel="Gruppenübergaben werden geladen…"
            loading
          />
        }
      >
        <SubstitutionPageContent />
      </Suspense>
    </RoleGuard>
  );
}
