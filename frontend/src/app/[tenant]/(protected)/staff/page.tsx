"use client";

import { useState, useEffect, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import {
  staffMonthCloseService,
  staffService,
  type Staff,
} from "~/lib/staff-api";
import {
  getStaffDisplayType,
  getStaffCardInfo,
  formatStaffNotes,
  sortStaff,
  getStaffLocationStatus,
} from "~/lib/staff-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useTenantRouter } from "~/lib/tenant-router";
import { isAdmin, hasPermission } from "~/lib/auth-utils";
import {
  StaffPendingInbox,
  useStaffPendingInbox,
} from "~/components/staff/staff-pending-inbox";
import { SchoolOverviewSection } from "~/components/staff/school-overview-section";
import { StaffAuditLog } from "~/components/staff/staff-audit-log";
import {
  StaffTimeAccountsTable,
  saldoPresets,
  type SaldoPresetId,
} from "~/components/staff/staff-time-accounts-table";
import { MonthCloseReasonModal } from "~/components/staff/month-close-modal";
import { StaffTimeExportModal } from "~/components/staff/staff-time-export-modal";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useSWRConfig } from "swr";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { staffOverviewService } from "~/lib/staff-overview-api";
import { employmentTypeLabels } from "~/lib/staff-helpers";
import { StaffPageSkeleton } from "./page-skeleton";

function StaffPageContent() {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const router = useTenantRouter();
  const userIsAdmin = isAdmin(session);

  // Die Zeitkonten-Ansicht ist an time_tracking:manage gebunden, nicht an
  // users:read: sie zeigt Arbeitszeitdaten identifizierbarer Personen.
  const canManageTimeTracking = hasPermission(session, "time_tracking:manage");
  const canReadUsers = hasPermission(session, "users:read");

  // State variables for filters
  const [searchTerm, setSearchTerm] = useState("");
  const [locationFilter, setLocationFilter] = useState("all");
  const [isMobile, setIsMobile] = useState(false);
  const [selectedView, setSelectedView] = useState<
    "status" | "accounts" | "audit"
  >("status");
  const [employmentFilter, setEmploymentFilter] = useState("all");
  // Angezeigter Monat der Zeitkonten (#1417): Default ist der laufende Monat;
  // zurückblättern erlaubt der Endpoint seit #1988, vorwärts über heute hinaus
  // lehnt er ab (Zukunftsmonat = volles Soll gegen null Ist).
  const berlinToday = useBerlinToday();
  const currentYear = Number(berlinToday.slice(0, 4));
  const currentMonth = Number(berlinToday.slice(5, 7));
  const [monthAnchor, setMonthAnchor] = useState<{
    year: number;
    month: number;
  }>(() => ({ year: currentYear, month: currentMonth }));
  const [showCloseModal, setShowCloseModal] = useState(false);
  const [showExportModal, setShowExportModal] = useState(false);
  const [saldoPreset, setSaldoPreset] = useState<SaldoPresetId>("all");
  const [customSaldoHours, setCustomSaldoHours] = useState("");
  const [showCustomSaldo, setShowCustomSaldo] = useState(false);
  // Ohne users:read ist die Statusansicht nicht erlaubt. Manage-only-Konten
  // landen deshalb direkt in den Zeitkonten, können aber zwischen Zeitkonten
  // und Änderungsprotokoll wechseln.
  const view =
    canReadUsers || selectedView === "audit" ? selectedView : "accounts";

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  // Fetch staff data with SWR (automatic caching, deduplication, revalidation)
  // Global SSE in TenantAuthWrapper handles cache invalidation automatically
  const {
    data: staffData,
    isLoading,
    error: staffError,
  } = useSWRAuth<Staff[]>(
    canReadUsers ? "staff-list" : null,
    async () => {
      const staffData = await staffService.getAllStaff({});
      return sortStaff(staffData);
    },
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
    },
  );

  const staff = staffData ?? [];
  const error = staffError ? "Fehler beim Laden der Personaldaten." : null;

  // Zeitkonten (#1417 Tranche 2a). Beschäftigungstyp und Saldo-Grenzen gehen
  // an den Server — der Beschäftigungstyp spart dort die Rechnung pro Person,
  // die Saldo-Grenzen verkleinern die Antwort. Sortiert und nach Namen
  // gefiltert wird im Client, das spart bei jedem Spaltenklick einen Roundtrip.
  const saldoBounds =
    saldoPresets.find((preset) => preset.id === saldoPreset) ?? saldoPresets[0];
  const customSaldoMinutes =
    customSaldoHours.trim() === "" || Number.isNaN(Number(customSaldoHours))
      ? undefined
      : Math.round(Number(customSaldoHours) * 60);
  // Eine eigene Untergrenze ersetzt das Preset vollständig, statt sich mit
  // dessen Obergrenze zu einem Bereich zu vermischen, den niemand so gewählt
  // hat.
  const saldoMin = customSaldoMinutes ?? saldoBounds.min ?? undefined;
  const saldoMax =
    customSaldoMinutes !== undefined
      ? undefined
      : (saldoBounds.max ?? undefined);

  const needsAuditStaffOptions =
    canManageTimeTracking && view === "audit" && !canReadUsers;
  const accountsKey =
    canManageTimeTracking && view === "accounts"
      ? `staff-time-accounts-${monthAnchor.year}-${monthAnchor.month}-${employmentFilter}-${saldoMin ?? ""}-${saldoMax ?? ""}`
      : needsAuditStaffOptions
        ? `staff-audit-options-${monthAnchor.year}-${monthAnchor.month}`
        : null;
  const {
    data: accounts,
    isLoading: accountsLoading,
    error: accountsError,
  } = useSWRAuth(
    accountsKey,
    () => {
      const month = {
        year: monthAnchor.year,
        month: monthAnchor.month,
      };
      if (needsAuditStaffOptions) {
        return staffOverviewService.getTimeAccounts(month);
      }
      return staffOverviewService.getTimeAccounts({
        ...month,
        employmentType:
          employmentFilter === "all" ? undefined : employmentFilter,
        saldoMin,
        saldoMax,
      });
    },
    // Filters update immediately. Keep the old rows out of the table until
    // the response for the active filter key arrives.
    { keepPreviousData: false, revalidateOnFocus: false },
  );

  // Abschluss-Status des angezeigten Monats (#1417). Leeres Array = offen.
  const monthCloseKey =
    canManageTimeTracking && view === "accounts"
      ? `staff-month-close-${monthAnchor.year}-${monthAnchor.month}`
      : null;
  const {
    data: monthCloseSnapshots,
    error: monthCloseStatusError,
    mutate: mutateMonthClose,
  } = useSWRAuth(
    monthCloseKey,
    () => staffMonthCloseService.getStatus(monthAnchor.year, monthAnchor.month),
    { keepPreviousData: false, revalidateOnFocus: false },
  );
  const monthClose =
    !monthCloseStatusError && monthCloseSnapshots
      ? {
          closed: monthCloseSnapshots.length > 0,
          closedAt: monthCloseSnapshots[0]?.closedAt ?? "",
          closedCount: monthCloseSnapshots.length,
        }
      : null;

  const isCurrentOrFutureMonth =
    monthAnchor.year > currentYear ||
    (monthAnchor.year === currentYear && monthAnchor.month >= currentMonth);
  const shiftMonth = (delta: number) => {
    setMonthAnchor((anchor) => {
      const date = new Date(anchor.year, anchor.month - 1 + delta, 1);
      return { year: date.getFullYear(), month: date.getMonth() + 1 };
    });
  };

  const { mutate: globalMutate } = useSWRConfig();
  const handleCloseMonth = async (reason: string) => {
    await staffMonthCloseService.closeMonth({
      year: monthAnchor.year,
      month: monthAnchor.month,
      reason,
    });
    await mutateMonthClose();
    // Folgemonate rechnen ab jetzt vom eingefrorenen Übertrag — die schulweite
    // Tabelle und beide Monatskarten-Familien sind potenziell veraltet.
    // includes statt startsWith: useSWRAuth präfixt die Keys mit dem Tenant.
    await globalMutate(
      (key) =>
        typeof key === "string" &&
        (key.includes("staff-time-accounts") ||
          key.includes("staff-month-summary-") ||
          key.includes("time-tracking-month-summary-")),
    );
  };

  const accountRows = useMemo(() => {
    const rows = accounts?.rows ?? [];
    if (!searchTerm) return rows;
    const needle = searchTerm.toLowerCase();
    return rows.filter((row) => row.name.toLowerCase().includes(needle));
  }, [accounts, searchTerm]);
  const auditStaffOptions = useMemo(
    () =>
      canReadUsers
        ? (staffData ?? []).map((row) => ({ id: row.id, name: row.name }))
        : (accounts?.rows ?? []).map((row) => ({
            id: row.staffId,
            name: row.name,
          })),
    [accounts?.rows, canReadUsers, staffData],
  );

  // Open absence requests (#1419): feeds the inbox above the grid and the
  // per-card pending indicators. Fetches only with vacation:approve.
  const { rows: pendingAbsences, canReview: canReviewAbsences } =
    useStaffPendingInbox();
  const pendingByStaff = useMemo(() => {
    const map = new Map<number, number>();
    for (const row of pendingAbsences) {
      map.set(row.staff_id, (map.get(row.staff_id) ?? 0) + 1);
    }
    return map;
  }, [pendingAbsences]);

  // Known non-room locations for the "Im Raum" filter
  const nonRoomLocations = new Set([
    "Zuhause",
    "Anwesend",
    "Schulhof",
    "Unterwegs",
    "Homeoffice",
    "Krank",
    "Urlaub",
    "Fortbildung",
    "Freizeitausgleich",
    "Abwesend",
  ]);

  // All absence location labels for the unified "Abwesend" filter
  const absenceLocations = new Set([
    "Krank",
    "Urlaub",
    "Fortbildung",
    "Freizeitausgleich",
    "Abwesend",
  ]);

  // Helper to check if location matches filter
  const matchesLocationFilter = (location: string, filter: string): boolean => {
    if (filter === "all") return true;
    // "abwesend_nicht_eingecheckt" - not clocked in (Zuhause or Abwesend)
    if (filter === "abwesend_nicht_eingecheckt")
      return location === "Zuhause" || location === "Abwesend";
    if (filter === "anwesend") return location === "Anwesend";
    if (filter === "homeoffice") return location === "Homeoffice";
    // "abwesend" - all absence types.
    if (filter === "abwesend") return absenceLocations.has(location);
    if (filter === "im_raum") {
      // Staff actively supervising in a room
      return !nonRoomLocations.has(location);
    }
    return true;
  };

  // Apply client-side filters
  const filteredStaff = staff.filter((staffMember) => {
    // Search filter
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      const matchesSearch =
        staffMember.firstName.toLowerCase().includes(searchLower) ||
        staffMember.lastName.toLowerCase().includes(searchLower) ||
        staffMember.name.toLowerCase().includes(searchLower);

      if (!matchesSearch) return false;
    }

    // Location filter
    const location = staffMember.currentLocation ?? "Abwesend";
    return matchesLocationFilter(location, locationFilter);
  });

  // Prepare filter configurations for PageHeaderWithSearch. Der Status-Filter
  // gehört zur Karten-Ansicht, der Beschäftigungstyp zur Zeitkonten-Tabelle —
  // beide gleichzeitig anzuzeigen würde Filter anbieten, die auf die sichtbare
  // Ansicht nicht wirken.
  const employmentFilterConfig: FilterConfig = useMemo(
    () => ({
      id: "employment",
      label: "Beschäftigung",
      type: "grid",
      value: employmentFilter,
      onChange: (value) => setEmploymentFilter(value as string),
      options: [
        { value: "all", label: "Alle", icon: "M4 6h16M4 12h16M4 18h16" },
        ...Object.entries(employmentTypeLabels).map(([value, label]) => ({
          value,
          label,
          icon: "M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0",
        })),
      ],
    }),
    [employmentFilter],
  );

  const locationFilterConfig: FilterConfig = useMemo(
    () => ({
      id: "location",
      label: "Status",
      type: "grid",
      value: locationFilter,
      onChange: (value) => setLocationFilter(value as string),
      options: [
        { value: "all", label: "Alle", icon: "M4 6h16M4 12h16M4 18h16" },
        {
          value: "anwesend",
          label: "Anwesend",
          icon: "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z",
        },
        {
          value: "abwesend_nicht_eingecheckt",
          label: "Abwesend",
          icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
        },
        {
          value: "im_raum",
          label: "Im Raum",
          icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
        },
        {
          value: "homeoffice",
          label: "Homeoffice",
          icon: "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
        },
        {
          value: "abwesend",
          label: "Krank/Urlaub",
          icon: "M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636",
        },
      ],
    }),
    [locationFilter],
  );

  // Das Änderungsprotokoll bringt seine eigenen Filter mit (MA, Editor,
  // Bereich, Zeitraum) — Header-Filter würden dort ins Leere laufen.
  const filterConfigs: FilterConfig[] =
    view === "audit"
      ? []
      : view === "accounts"
        ? [employmentFilterConfig]
        : [locationFilterConfig];

  // Prepare active filters for display
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (view === "accounts") {
      if (employmentFilter !== "all") {
        filters.push({
          id: "employment",
          label: employmentTypeLabels[employmentFilter] ?? employmentFilter,
          onRemove: () => setEmploymentFilter("all"),
        });
      }
      if (customSaldoHours.trim() !== "") {
        filters.push({
          id: "saldo",
          label: `Saldo ab ${customSaldoHours} Std.`,
          onRemove: () => {
            setCustomSaldoHours("");
            setShowCustomSaldo(false);
          },
        });
      } else if (saldoPreset !== "all") {
        const preset = saldoPresets.find((entry) => entry.id === saldoPreset);
        filters.push({
          id: "saldo",
          label: preset?.label ?? saldoPreset,
          onRemove: () => setSaldoPreset("all"),
        });
      }
      return filters;
    }

    if (locationFilter !== "all") {
      const locationLabels: Record<string, string> = {
        anwesend: "Anwesend",
        abwesend_nicht_eingecheckt: "Abwesend",
        im_raum: "Im Raum",
        homeoffice: "Homeoffice",
        abwesend: "Krank/Urlaub",
      };
      filters.push({
        id: "location",
        label: locationLabels[locationFilter] ?? locationFilter,
        onRemove: () => setLocationFilter("all"),
      });
    }

    return filters;
  }, [
    searchTerm,
    locationFilter,
    employmentFilter,
    saldoPreset,
    customSaldoHours,
    view,
  ]);

  if (status === "loading" || isLoading) {
    return <StaffPageSkeleton />;
  }

  if (!canReadUsers && !canManageTimeTracking) {
    return <ForbiddenPage />;
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* PageHeaderWithSearch - Title only on mobile */}
      <PageHeaderWithSearch
        title={isMobile ? "Mitarbeiter" : ""}
        badge={{
          icon: (
            <svg
              className="h-5 w-5 text-gray-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2"
              />
            </svg>
          ),
          count:
            view === "accounts" ? accountRows.length : filteredStaff.length,
        }}
        search={
          // Im Änderungsprotokoll filtert die Komponente selbst; ein
          // wirkungsloses Suchfeld wäre irreführend.
          view === "audit"
            ? undefined
            : {
                value: searchTerm,
                onChange: setSearchTerm,
                placeholder: "Name suchen...",
              }
        }
        filters={filterConfigs}
        activeFilters={activeFilters}
        onClearAllFilters={() => {
          setSearchTerm("");
          setLocationFilter("all");
          setEmploymentFilter("all");
          setSaldoPreset("all");
          setCustomSaldoHours("");
          setShowCustomSaldo(false);
        }}
      />

      {/* Sektion 1 — Eingehende Anfragen (#1419), nur für Genehmigende. Die
          Inbox trägt die offenen Anträge samt Detail; das aggregierte
          pending_requests_count aus dem Dashboard-Endpoint wäre daneben eine
          zweite, ärmere Anzeige derselben Zahl und bleibt deshalb ungenutzt. */}
      {canReviewAbsences && (
        <StaffPendingInbox rows={pendingAbsences} staffList={staff} />
      )}

      {/* Sektion 2 — Einrichtungs-Übersicht (#1417 Tranche 2a), läuft mit users:read */}
      {canReadUsers && <SchoolOverviewSection />}

      {/* Sektion 3 — Mitarbeitende. Status (Karten) und Zeitkonten (Tabelle)
          beantworten verschiedene Fragen; der Umschalter erscheint nur mit
          time_tracking:manage. */}
      {canManageTimeTracking && (
        <Tabs
          value={view}
          onValueChange={(value) =>
            setSelectedView(value as "status" | "accounts" | "audit")
          }
          className="mb-4"
        >
          <TabsList variant="line">
            {canReadUsers && <TabsTrigger value="status">Status</TabsTrigger>}
            <TabsTrigger value="accounts">Zeitkonten</TabsTrigger>
            <TabsTrigger value="audit">Änderungsprotokoll</TabsTrigger>
          </TabsList>
        </Tabs>
      )}

      {/* Änderungsprotokoll (#1417): cross-MA audit feed, eigene Filter in der
          Komponente. Nur mit time_tracking:manage erreichbar (Tab-Gate oben). */}
      {view === "audit" && canManageTimeTracking && (
        <StaffAuditLog staffOptions={auditStaffOptions} />
      )}

      {view === "accounts" && (
        <StaffTimeAccountsTable
          rows={accountRows}
          year={accounts?.year ?? monthAnchor.year}
          month={accounts?.month ?? monthAnchor.month}
          onPrevMonth={() => shiftMonth(-1)}
          onNextMonth={() => shiftMonth(1)}
          canGoNextMonth={!isCurrentOrFutureMonth}
          monthClose={monthClose}
          monthCloseError={
            monthCloseStatusError
              ? "Der Abschlussstatus konnte nicht geladen werden. Abschließen und erneutes Abschließen sind bis zur erfolgreichen Aktualisierung nicht verfügbar."
              : null
          }
          onRetryMonthClose={() => void mutateMonthClose()}
          monthIsOver={!isCurrentOrFutureMonth}
          onCloseMonth={() => setShowCloseModal(true)}
          onExport={() => setShowExportModal(true)}
          onOpeningBalances={
            canManageTimeTracking
              ? () => router.push("/database/personal/opening-balances")
              : undefined
          }
          isLoading={accountsLoading && !accounts}
          error={
            accountsError ? "Zeitkonten konnten nicht geladen werden." : null
          }
          onRowClick={(row) => router.push(`/staff/${row.staffId}`)}
          saldoPreset={saldoPreset}
          onSaldoPresetChange={setSaldoPreset}
          customSaldoHours={customSaldoHours}
          onCustomSaldoHoursChange={setCustomSaldoHours}
          showCustomSaldo={showCustomSaldo}
          onShowCustomSaldoChange={setShowCustomSaldo}
          paginationResetKey={`${accountsKey ?? "inactive"}:${searchTerm}`}
        />
      )}

      {showCloseModal && (
        <MonthCloseReasonModal
          title={`Monat abschließen — ${String(monthAnchor.month).padStart(2, "0")}/${monthAnchor.year}`}
          description={
            <>
              <p>
                Der Abschluss friert den Saldo zum Monatsende für{" "}
                <strong>alle Mitarbeitenden</strong> ein. Spätere Monate rechnen
                ab dann mit diesem eingefrorenen Übertrag weiter.
              </p>
              <p>
                Werden danach noch Zeiten in diesem Monat geändert, bleibt der
                Übertrag stehen; die Abweichung wird auf der Monatskarte der
                Person angezeigt.
              </p>
              <p>
                Bereits abgeschlossene Konten bleiben unverändert; nur offene
                Konten werden eingefroren.
              </p>
            </>
          }
          submitLabel="Monat abschließen"
          successMessage="Monat abgeschlossen."
          onSubmit={handleCloseMonth}
          onClose={() => setShowCloseModal(false)}
        />
      )}

      {/* Cross-MA-Export (#1417 2b): Query-Bau im Dialog, Zahlen und Datei
          komplett aus dem Backend. */}
      <StaffTimeExportModal
        isOpen={showExportModal}
        onClose={() => setShowExportModal(false)}
        year={accounts?.year ?? monthAnchor.year}
        month={accounts?.month ?? monthAnchor.month}
      />

      {/* Error Display */}
      {view === "status" && error && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
          {error}
        </div>
      )}

      {/* Staff Grid */}
      {view === "status" &&
        (filteredStaff.length === 0 ? (
          <div className="py-12 text-center">
            <div className="flex flex-col items-center gap-4">
              <svg
                className="h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2"
                />
              </svg>
              <div>
                <h3 className="text-lg font-medium text-gray-900">
                  Kein Personal gefunden
                </h3>
                <p className="text-gray-600">
                  Versuchen Sie Ihre Suchkriterien anzupassen.
                </p>
              </div>
            </div>
          </div>
        ) : (
          <div>
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
              {filteredStaff.map((staffMember) => {
                const locationStatus = getStaffLocationStatus(staffMember);
                const displayType = getStaffDisplayType(staffMember);
                const cardInfo = getStaffCardInfo(staffMember);
                const notes = formatStaffNotes(staffMember.staffNotes, 80);
                const supervisions = staffMember.supervisions ?? [];

                const cardClassName = `group moto-content-surface moto-hover-elevated relative w-full overflow-hidden rounded-2xl border border-gray-200 bg-white text-left shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:ring-offset-2 focus-visible:outline-none active:shadow-[0_10px_26px_rgba(15,23,42,0.1)] ${userIsAdmin ? "cursor-pointer" : ""}`;
                const navigateToStaff = () =>
                  router.push(`/staff/${staffMember.id}`);

                return (
                  <div
                    key={staffMember.id}
                    {...(userIsAdmin
                      ? {
                          role: "button" as const,
                          tabIndex: 0,
                          onClick: navigateToStaff,
                          onKeyDown: (e: React.KeyboardEvent) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              navigateToStaff();
                            }
                          },
                        }
                      : {})}
                    className={cardClassName}
                  >
                    <div className="relative p-4">
                      <div className="relative flex min-h-[104px] flex-col">
                        <div className="mb-3 flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            {/* Ein Name, eine Zeile: der Umbruch zwischen Vor-
                                und Nachname ließ jede Karte wie zwei Personen
                                aussehen. */}
                            <div className="flex min-w-0 items-center gap-1.5">
                              <h3 className="truncate text-base font-bold text-gray-900">
                                {staffMember.firstName} {staffMember.lastName}
                              </h3>
                              {userIsAdmin && (
                                <svg
                                  className="h-4 w-4 flex-shrink-0 text-gray-300 transition-colors duration-200 md:group-hover:text-gray-500"
                                  fill="none"
                                  viewBox="0 0 24 24"
                                  stroke="currentColor"
                                >
                                  <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth={2}
                                    d="M9 5l7 7-7 7"
                                  />
                                </svg>
                              )}
                            </div>
                            <p className="mt-0.5 truncate text-xs text-gray-500">
                              {staffMember.specialization ?? displayType}
                            </p>
                          </div>

                          <span className="flex flex-shrink-0 flex-col items-end gap-1.5">
                            {/* Kein Glow, kein Pulsieren: 20 gleichzeitig
                                leuchtende Badges sind Lärm, kein Status. */}
                            <span
                              className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold ${locationStatus.badgeColor}`}
                              style={{
                                backgroundColor: locationStatus.customBgColor,
                              }}
                            >
                              <span className="h-1.5 w-1.5 rounded-full bg-white/80"></span>
                              {locationStatus.label}
                            </span>
                            {(pendingByStaff.get(Number(staffMember.id)) ?? 0) >
                              0 && (
                              <span className="inline-flex items-center rounded-full bg-red-500 px-2 py-0.5 text-[11px] font-bold text-white">
                                {pendingByStaff.get(Number(staffMember.id))}{" "}
                                {pendingByStaff.get(Number(staffMember.id)) ===
                                1
                                  ? "Anfrage"
                                  : "Anfragen"}
                              </span>
                            )}
                          </span>
                        </div>

                        <div className="flex-1 space-y-2">
                          {supervisions.length > 0 && (
                            <div className="text-sm text-gray-600">
                              <span className="font-medium">
                                Aktuelle Aufsicht:
                              </span>
                              <ul className="mt-0.5 list-inside">
                                {supervisions.map((supervision) => (
                                  <li
                                    key={`${supervision.roomId}-${supervision.activeGroupId}`}
                                  >
                                    • {supervision.roomName}
                                  </li>
                                ))}
                              </ul>
                            </div>
                          )}

                          {cardInfo.length > 0 && (
                            <div className="flex flex-wrap gap-2">
                              {cardInfo.map((info) => (
                                <span
                                  key={info}
                                  className="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700"
                                >
                                  {info}
                                </span>
                              ))}
                            </div>
                          )}

                          {notes && (
                            <p className="text-xs text-gray-500 italic transition-colors duration-300 md:group-hover:text-gray-600">
                              {notes}
                            </p>
                          )}
                        </div>

                        {userIsAdmin && (
                          <p className="mt-2 text-xs text-gray-400 transition-colors duration-150 md:group-hover:text-gray-500">
                            Tippen für mehr Infos
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        ))}
    </div>
  );
}

export default function StaffPage() {
  return <StaffPageContent />;
}
