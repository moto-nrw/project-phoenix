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
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
import { Skeleton } from "~/components/ui/skeleton";
import {
  SkeletonRegion,
  PageHeaderSkeleton,
  ListSkeleton,
} from "~/components/ui/page-skeletons";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useToast } from "~/contexts/ToastContext";
import { groupService } from "~/lib/api";
import type { Group } from "~/lib/api";
import { formatDate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { substitutionService } from "~/lib/substitution-api";
import type {
  Substitution,
  TeacherAvailability,
} from "~/lib/substitution-helpers";
import {
  formatTeacherName,
  getTeacherStatus,
} from "~/lib/substitution-helpers";
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
  { value: "1", label: "Heute" },
  { value: "3", label: "3 Tage" },
  { value: "7", label: "1 Woche" },
  { value: CUSTOM_DURATION, label: "Individuell" },
];

const MAX_DURATION_DAYS = 365;

/**
 * Enddatum eines heute beginnenden Zugriffs. Der Starttag zählt mit, ein
 * Zugriff über einen Tag endet also heute — dieselbe Rechnung, die auch an das
 * Backend geht.
 */
function accessEndDate(days: number): Date {
  const end = new Date();
  end.setDate(end.getDate() + days - 1);
  return end;
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

/** Gemeinsame Listenfläche: eine weiße Karte mit Trennlinien statt einzelner Karten. */
function ListSurface({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <ul className="moto-content-surface divide-y divide-gray-200 overflow-hidden rounded-2xl border shadow-sm">
      {children}
    </ul>
  );
}

function SectionHeading({
  icon,
  title,
  count,
  hint,
}: Readonly<{
  icon: ReactNode;
  title: string;
  count: number;
  hint?: string;
}>) {
  return (
    <div className="mb-3 flex flex-wrap items-center gap-2 md:mb-4">
      <span className="text-gray-500" aria-hidden="true">
        {icon}
      </span>
      <h2 className="text-base font-semibold text-gray-900">{title}</h2>
      <span className="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-gray-100 px-2 text-xs font-semibold text-gray-700 tabular-nums">
        {count}
      </span>
      {hint ? <span className="text-xs text-gray-500">{hint}</span> : null}
    </div>
  );
}

/**
 * Eine Zeile in den beiden Zugriffs-Abschnitten. Die Art des Zugriffs steht
 * schon in der Abschnittsüberschrift, deshalb trägt die Zeile sie nicht noch
 * einmal als Kennzeichnung; nur das Enddatum kommt hinzu, wo es eines gibt.
 */
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
        <MetaLine parts={[`Zugriff für ${personName}`, until]} />
      </div>
      <Button
        type="button"
        variant="outline_danger"
        size="md"
        onClick={onEnd}
        disabled={disabled}
        aria-label={`Zugriff von ${personName} auf ${groupName} beenden`}
        className="w-full sm:w-auto"
      >
        Beenden
      </Button>
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

  // Gruppenzugriff ist nur bei festen Gruppen sinnvoll (#1940); bei offener
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
    () => substitutionService.fetchAvailableTeachers(new Date()),
    { keepPreviousData: true },
  );

  const { data: groups = [] } = useImmutableSWR<Group[]>(
    "substitution-groups",
    () => groupService.getGroups(),
  );

  const { data: activeSubstitutions = [], mutate: mutateActiveSubstitutions } =
    useSWRAuth<Substitution[]>(
      "active-substitutions",
      () => substitutionService.fetchActiveSubstitutions(new Date()),
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
  const [durationPreset, setDurationPreset] = useState("1");
  const [customDays, setCustomDays] = useState(2);

  // Beenden-Bestätigung
  const [showEndConfirmation, setShowEndConfirmation] = useState(false);
  const [substitutionToEnd, setSubstitutionToEnd] = useState<{
    id: string;
    groupName: string;
    teacherName: string;
  } | null>(null);

  const isLoading = teachersLoading;
  const loadError = teachersError ? "Fehler beim Laden der Daten." : null;

  const substitutionDays =
    durationPreset === CUSTOM_DURATION ? customDays : Number(durationPreset);

  const filteredTeachers = useMemo(() => {
    let filtered = [...teachers];

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
  }, [teachers, searchTerm, statusFilter]);

  const openSubstitutionPopup = (teacher: TeacherAvailability) => {
    setSelectedTeacher(teacher);
    setSelectedGroupId("");
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
    if (!selectedTeacher || !selectedGroupId) {
      setMutationError("Bitte wähle eine Gruppe aus.");
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

      // Kein regular_staff_id: der Zugriff ersetzt niemanden, er kommt zusätzlich
      // zu den bestehenden Berechtigungen der Gruppe.
      const regularStaffId = null;

      await substitutionService.createSubstitution(
        group.id,
        regularStaffId,
        selectedTeacher.id,
        new Date(),
        accessEndDate(substitutionDays),
        "Gruppenzugriff",
        `Gruppenzugriff für ${substitutionDays} Tag(e)`,
      );

      await Promise.all([mutateTeachers(), mutateActiveSubstitutions()]);

      const teacherName = formatTeacherName(selectedTeacher);
      const days = substitutionDays > 1 ? `${substitutionDays} Tage` : "1 Tag";
      showSuccessToast(
        `Zugriff auf "${group.name}" für ${teacherName} gewährt (${days})`,
      );

      closePopup();
    } catch (err) {
      logger.error("failed to create substitution", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMutationError("Fehler beim Gewähren des Zugriffs.");
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

      showSuccessToast(`Zugriff auf "${substitutionToEnd.groupName}" beendet`);

      setShowEndConfirmation(false);
      setSubstitutionToEnd(null);
    } catch (err) {
      logger.error("failed to end substitution", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMutationError("Fehler beim Beenden des Zugriffs.");
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
          { value: "substitution", label: "Hat Zugriff" },
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
        substitution: "Hat Zugriff",
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
    return <SubstitutionPageSkeleton />;
  }

  if (openCareGroupMode) {
    return (
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        {/* Kein Erklärabsatz unter einer Überschrift, sondern ein Leerzustand. */}
        <EmptyState
          icon={<MotoConceptIcon concept="staff" size={48} />}
          title="Gruppenzugriff nicht verfügbar"
          description="Diese Schule arbeitet mit offener Betreuung ohne feste Gruppen. Alle berechtigten Mitarbeitenden arbeiten mit allen Kindern, daher ist kein temporärer Gruppenzugriff nötig. Die Einstellung „Arbeit mit festen Gruppen“ kann in den Einstellungen geändert werden."
        />
      </div>
    );
  }

  const transfers = activeSubstitutions.filter((s) => s.isTransfer);
  const longTermAccess = activeSubstitutions.filter((s) => !s.isTransfer);

  const renderTeacherList = () => {
    if (isLoading) {
      return (
        <SkeletonRegion label="Fachkräfte werden geladen…">
          <ListSkeleton rows={6} />
        </SkeletonRegion>
      );
    }

    if (filteredTeachers.length === 0) {
      return (
        <EmptyState
          icon={<MotoConceptIcon concept="staff" size={48} />}
          title="Keine Fachkräfte gefunden"
          description="Passen Sie Ihre Suchkriterien an."
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
                        ? getTeacherStatus(teacher)
                        : null,
                    ]}
                  />
                </div>
              </div>
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => openSubstitutionPopup(teacher)}
                aria-label={`${name} Zugriff auf eine Gruppe geben`}
                className="w-full sm:w-auto"
              >
                Zuweisen
              </Button>
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
      return (
        <div className="moto-content-surface rounded-2xl border p-4 text-sm text-gray-500">
          {emptyText}
        </div>
      );
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

  return (
    <>
      <div className="w-full">
        {/* Kopfkarte auf allen Breakpoints, wie in der Eltern-App. */}
        <PageIntro
          title="Gruppenzugriff"
          description={
            // Statuszeile: verfügbare Fachkräfte und aktive Vertretungen aus
            // den beiden Listen, die die Seite ohnehin lädt.
            isLoading ? (
              <Skeleton className="h-4 w-56" />
            ) : (
              `${teachers.length} ${teachers.length === 1 ? "Fachkraft" : "Fachkräfte"} · ${activeSubstitutions.length} ${
                activeSubstitutions.length === 1
                  ? "Vertretung aktiv"
                  : "Vertretungen aktiv"
              }`
            )
          }
          className="mb-6"
        >
          <PageHeaderWithSearch
            embedded
            title=""
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
          />
        </PageIntro>

        {loadError && (
          <div className="mb-6">
            <Alert type="error" message={loadError} />
          </div>
        )}

        {/* Die Anzahl steht bereits im Zähler des Seitenkopfs, deshalb hier
            eine schlichte Überschrift ohne Zählpille. */}
        <div className="mb-6">
          <h2 className="mb-3 text-base font-semibold text-gray-900 md:mb-4">
            Verfügbare pädagogische Fachkräfte
          </h2>
          {renderTeacherList()}
        </div>

        <div className="space-y-6">
          <section>
            <SectionHeading
              icon={<Clock className="h-5 w-5" />}
              title="Tagesübergaben"
              count={transfers.length}
              hint="(enden heute 23:59)"
            />
            {/* Kein Enddatum: alle Zeilen dieses Abschnitts enden heute, das
                steht bereits in der Überschrift. */}
            {renderAccessSection(
              transfers,
              "Keine aktiven Tagesübergaben",
              () => null,
            )}
          </section>

          <section>
            <SectionHeading
              icon={<MotoConceptIcon concept="calendar" size={20} />}
              title="Längerfristige Zugriffe"
              count={longTermAccess.length}
              hint="(mehrtägig)"
            />
            {renderAccessSection(
              longTermAccess,
              "Keine aktiven längerfristigen Zugriffe",
              (substitution) =>
                `bis ${substitution.endDate.toLocaleDateString("de-DE", {
                  timeZone: "Europe/Berlin",
                  day: "2-digit",
                  month: "2-digit",
                })}`,
            )}
          </section>
        </div>
      </div>

      <Modal
        isOpen={showPopup}
        onClose={closePopup}
        title="Zugriff gewähren"
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
            erhält zusätzlichen Zugriff auf die Kinder der gewählten Gruppe. Die
            bestehenden Berechtigungen der Gruppe bleiben unverändert.
          </p>

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
            <Tabs value={durationPreset} onValueChange={setDurationPreset}>
              <TabsList
                variant="default"
                aria-labelledby="substitution-duration-label"
                className="h-auto flex-wrap"
              >
                {DURATION_PRESETS.map((preset) => (
                  <TabsTrigger key={preset.value} value={preset.value}>
                    {preset.label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            {durationPreset === CUSTOM_DURATION ? (
              <div className="mt-3 max-w-40">
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
                ? "Zugriff nur für heute, endet um 23:59 Uhr."
                : `Zugriff bis ${formatDate(toISODate(accessEndDate(substitutionDays)), true)}, 23:59 Uhr.`}
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
        title="Zugriff beenden?"
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
              Möchten Sie den Zugriff wirklich beenden? Die Person sieht die
              Kinder der Gruppe danach nicht mehr.
            </p>
            <div className="rounded-2xl border border-gray-200 bg-gray-50 p-4">
              <p className="text-sm text-gray-600">
                <span className="font-medium text-gray-900">Gruppe:</span>{" "}
                {substitutionToEnd.groupName}
              </p>
              <p className="mt-1 text-sm text-gray-600">
                <span className="font-medium text-gray-900">Zugriff für:</span>{" "}
                {substitutionToEnd.teacherName}
              </p>
            </div>
          </div>
        )}
      </ConfirmationModal>
    </>
  );
}

function SubstitutionPageSkeleton() {
  return (
    <SkeletonRegion label="Gruppenzugriff wird geladen">
      <PageHeaderSkeleton />
      <ListSkeleton rows={6} />
    </SkeletonRegion>
  );
}

export default function SubstitutionPage() {
  return (
    <RoleGuard variant="adminOnly">
      <Suspense fallback={<SubstitutionPageSkeleton />}>
        <SubstitutionPageContent />
      </Suspense>
    </RoleGuard>
  );
}
