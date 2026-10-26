"use client";

import { useEffect, useId, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import type { FormErrorInput } from "~/components/ui/form-error";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { parseLocation } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { useLocalStorageValue } from "~/lib/hooks/use-local-storage-value";
import { useMinuteClock } from "~/lib/pickup-helpers";
import {
  type CareOfferingOption,
  fetchCareOfferingOptions,
  fetchStudents,
} from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import {
  presentChildCandidates,
  type PresentChildScope,
} from "~/lib/timetable-roster-helpers";

const logger = createLogger({ component: "PresentChildrenPicker" });

const PRESENT_PAGE_SIZE = 100;

const BASE_SCOPE_ITEMS = [
  { value: "stays", label: "Noch in Betreuung" },
  { value: "all", label: "Alle anwesenden" },
] as const;

const SCOPE_ITEMS_WITH_OFFERING = [
  ...BASE_SCOPE_ITEMS,
  { value: "offering", label: "Nach Angebot" },
] as const;

/** The list a scope reads: every present child, or one offering's. */
const PRESENT_SOURCE = "present";

/**
 * Per school and account (#5000): another account on the same device opens
 * the dialog without this account's offering preselected.
 */
function offeringStorageKey(
  tenantSlug: string | null,
  accountId: string | undefined,
): string | null {
  return tenantSlug && accountId
    ? `moto:present-children-offering:${tenantSlug}:${accountId}`
    : null;
}

function storeOffering(key: string | null, offeringId: string | null) {
  if (!key || !offeringId) return;
  try {
    globalThis.localStorage.setItem(key, offeringId);
  } catch {
    // Storage blocked (private mode, policy): the choice is just not
    // remembered for the next opening.
  }
}

interface PresentChild {
  readonly id: string;
  readonly name: string;
  readonly detail: string;
  readonly place: string | null;
  readonly pickupTime: string | null;
}

async function fetchPresentChildren(
  careOfferingId: string | null,
): Promise<PresentChild[]> {
  const children: PresentChild[] = [];
  for (let page = 1; ; page += 1) {
    const result = await fetchStudents({
      location_state: "present",
      include_pickup_times: true,
      page,
      page_size: PRESENT_PAGE_SIZE,
      ...(careOfferingId ? { care_offering_id: careOfferingId } : {}),
    });
    children.push(...result.students.map(toPresentChild));

    if (!result.pagination || page >= result.pagination.total_pages) {
      return children;
    }
  }
}

function toPresentChild(student: Student): PresentChild {
  const parsed = parseLocation(student.current_location);
  return {
    id: student.id.toString(),
    name:
      student.name ||
      [student.first_name, student.second_name].filter(Boolean).join(" "),
    detail: [student.school_class, student.group_name]
      .filter(Boolean)
      .join(" · "),
    place: parsed.room ?? null,
    pickupTime: student.pickup_time ?? null,
  };
}

type LoadState =
  | { readonly kind: "loading" }
  | { readonly kind: "failed" }
  | { readonly kind: "loaded"; readonly children: readonly PresentChild[] };

type OfferingsState =
  | { readonly kind: "loading" }
  | { readonly kind: "loaded"; readonly items: readonly CareOfferingOption[] };

export interface PresentChildrenPickerProps {
  readonly isOpen: boolean;
  readonly instanceId: string;
  /** Kinder, die gerade schon in diesem Block sind; sie stehen nicht zur Wahl. */
  readonly inBlockStudentIds: ReadonlySet<string>;
  readonly isAdding: boolean;
  readonly error: FormErrorInput;
  readonly onAdd: (studentIds: string[]) => Promise<boolean>;
  readonly onClose: () => void;
}

/**
 * Schlägt die Kinder vor, die gerade in der OGS sind, und trägt eine Auswahl
 * gesammelt in den laufenden Block ein (#3824). „Noch in Betreuung“ blendet
 * Kinder aus, deren Gehzeit heute schon erreicht ist: sie gehen gleich und
 * gehören nicht mehr in eine neue Aktivität. „Nach Angebot“ zeigt die
 * Kinder, die ein Betreuungsangebot heute gebucht haben und nicht früher
 * abgemeldet sind (#4571); das Backend entscheidet die Liste, der Browser
 * merkt sich das zuletzt gewählte Angebot je Schule und wählt es beim
 * nächsten Öffnen vor.
 */
export function PresentChildrenPicker({
  isOpen,
  instanceId,
  inBlockStudentIds,
  isAdding,
  error,
  onAdd,
  onClose,
}: PresentChildrenPickerProps) {
  const formId = useId();
  const now = useMinuteClock();
  const [load, setLoad] = useState<LoadState>({ kind: "loading" });
  const [offerings, setOfferings] = useState<OfferingsState>({
    kind: "loading",
  });
  // null until the person chooses in this opening; until then the stored
  // offering, or else "Noch in Betreuung", applies.
  const [scopeChoice, setScopeChoice] = useState<PresentChildScope | null>(
    null,
  );
  const [offeringChoice, setOfferingChoice] = useState<string | null>(null);
  const { data: session } = useSession();
  const storageKey = offeringStorageKey(useTenantSlugSafe(), session?.user?.id);
  const storedOfferingId = useLocalStorageValue(
    storageKey ?? "",
    storageKey !== null,
  );
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  // Ein anderer Block bei offenem Dialog verwirft die Auswahl: sie darf nie
  // in einen anderen Block geschrieben werden.
  const [selectionInstanceId, setSelectionInstanceId] = useState(instanceId);
  if (selectionInstanceId !== instanceId) {
    setSelectionInstanceId(instanceId);
    setSelected(new Set());
  }

  const offeringItems = offerings.kind === "loaded" ? offerings.items : [];
  const storedOffering = offeringItems.some(
    (offering) => offering.id === storedOfferingId,
  )
    ? storedOfferingId
    : null;
  const offeringId =
    offeringChoice ?? storedOffering ?? offeringItems[0]?.id ?? null;
  const scope: PresentChildScope =
    scopeChoice ?? (storedOffering ? "offering" : "stays");
  const source =
    scope === "offering" && offeringId ? offeringId : PRESENT_SOURCE;

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    setOfferings({ kind: "loading" });
    fetchCareOfferingOptions()
      .then((items) => {
        if (!cancelled) setOfferings({ kind: "loaded", items });
      })
      .catch((err) => {
        if (cancelled) return;
        // The offering selection is an extra: without it the dialog works
        // as before, so a failure only hides "Nach Angebot".
        logger.warn("failed to load care offerings", {
          error: err instanceof Error ? err.message : String(err),
        });
        setOfferings({ kind: "loaded", items: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, instanceId]);

  const offeringsLoaded = offerings.kind === "loaded";
  useEffect(() => {
    if (!isOpen || !offeringsLoaded) return;
    let cancelled = false;
    setLoad({ kind: "loading" });
    fetchPresentChildren(source === PRESENT_SOURCE ? null : source)
      .then((result) => {
        if (cancelled) return;
        setLoad({
          kind: "loaded",
          children: result,
        });
      })
      .catch((err) => {
        if (cancelled) return;
        logger.warn("failed to load present children", {
          error: err instanceof Error ? err.message : String(err),
        });
        setLoad({ kind: "failed" });
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, instanceId, offeringsLoaded, source]);

  const candidates = useMemo(
    () =>
      load.kind === "loaded"
        ? presentChildCandidates(load.children, inBlockStudentIds, now, scope)
        : [],
    [load, inBlockStudentIds, now, scope],
  );
  const term = search.trim().toLocaleLowerCase("de");
  const visible = term
    ? candidates.filter((child) =>
        child.name.toLocaleLowerCase("de").includes(term),
      )
    : candidates;
  // Nur Kinder, die in der aktuell sichtbaren Auswahl noch zur Wahl stehen,
  // zählen. Ein Kind, das inzwischen im Block ist oder die aktuelle
  // Auswahlbedingung nicht mehr erfüllt, fällt aus der Auswahl.
  const offeredIds = new Set(candidates.map((child) => child.id));
  const selectedIds = [...selected].filter((id) => offeredIds.has(id));
  const allVisibleSelected =
    visible.length > 0 && visible.every((child) => selected.has(child.id));

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };
  const toggleAllVisible = () => {
    setSelected((prev) => {
      const next = new Set(prev);
      for (const child of visible) {
        if (allVisibleSelected) next.delete(child.id);
        else next.add(child.id);
      }
      return next;
    });
  };
  const changeScope = (nextScope: PresentChildScope) => {
    setScopeChoice(nextScope);
    if (nextScope === "offering") storeOffering(storageKey, offeringId);
    // Another list starts without a selection: a hidden choice must never
    // come back and be added unseen.
    if (nextScope === "offering" || scope === "offering") {
      if (nextScope !== scope) setSelected(new Set());
      return;
    }
    const nextOfferedIds = new Set(
      load.kind === "loaded"
        ? presentChildCandidates(
            load.children,
            inBlockStudentIds,
            now,
            nextScope,
          ).map((child) => child.id)
        : [],
    );
    setSelected(
      (prev) => new Set([...prev].filter((id) => nextOfferedIds.has(id))),
    );
  };
  const changeOffering = (nextOfferingId: string) => {
    if (nextOfferingId !== offeringId) setSelected(new Set());
    setOfferingChoice(nextOfferingId);
    storeOffering(storageKey, nextOfferingId);
  };
  const handleClose = () => {
    setSelected(new Set());
    setSearch("");
    setScopeChoice(null);
    setOfferingChoice(null);
    // The next opening loads the offerings first, then the children.
    setOfferings({ kind: "loading" });
    onClose();
  };
  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (selectedIds.length === 0 || isAdding) return;
    if (await onAdd(selectedIds)) handleClose();
  };

  let submitLabel = "Kinder hinzufügen";
  if (isAdding) submitLabel = "Wird hinzugefügt…";
  else if (selectedIds.length === 1) submitLabel = "1 Kind hinzufügen";
  else if (selectedIds.length > 1)
    submitLabel = `${selectedIds.length} Kinder hinzufügen`;

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleClose}
      title="Anwesende Kinder hinzufügen"
      size="md"
      error={error}
      closeDisabled={isAdding}
      footer={
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={handleClose}
            disabled={isAdding}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form={formId}
            disabled={isAdding || selectedIds.length === 0}
            variant="success"
            size="md"
          >
            {submitLabel}
          </Button>
        </div>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="space-y-3">
        <p className="text-sm text-gray-600">
          Diese Kinder sind gerade in der OGS. Gewählte Kinder kommen sofort in
          diese Aktivität.
        </p>
        <SegmentedControl
          items={
            offeringItems.length > 0
              ? SCOPE_ITEMS_WITH_OFFERING
              : BASE_SCOPE_ITEMS
          }
          value={scope}
          onChange={changeScope}
          fullWidth
          className={
            offeringItems.length > 0 ? "max-sm:grid-flow-row" : undefined
          }
          ariaLabel="Welche Kinder zeigen"
        />
        {scope === "offering" && offeringId ? (
          <div className="space-y-1">
            <CustomSelect
              value={offeringId}
              options={offeringItems.map((offering) => ({
                value: offering.id,
                label: offering.name,
              }))}
              onChange={changeOffering}
              ariaLabel="Betreuungsangebot"
            />
            <p className="text-sm text-gray-600">
              Ohne Kinder, die heute früher gehen oder abgemeldet sind.
            </p>
          </div>
        ) : null}
        <Input
          type="search"
          name="present-children-search"
          aria-label="Anwesende Kinder durchsuchen"
          controlSize="compact"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Name suchen..."
        />
        <PresentChildrenList
          load={load}
          visible={visible}
          scope={scope}
          hasSearch={term.length > 0}
          selected={selected}
          isAdding={isAdding}
          allVisibleSelected={allVisibleSelected}
          onToggle={toggle}
          onToggleAll={toggleAllVisible}
        />
      </form>
    </FormModal>
  );
}

interface PresentChildrenListProps {
  readonly load: LoadState;
  readonly visible: readonly PresentChild[];
  readonly scope: PresentChildScope;
  readonly hasSearch: boolean;
  readonly selected: ReadonlySet<string>;
  readonly isAdding: boolean;
  readonly allVisibleSelected: boolean;
  readonly onToggle: (id: string) => void;
  readonly onToggleAll: () => void;
}

function PresentChildrenList({
  load,
  visible,
  scope,
  hasSearch,
  selected,
  isAdding,
  allVisibleSelected,
  onToggle,
  onToggleAll,
}: PresentChildrenListProps) {
  if (load.kind === "loading") {
    return <Loading message="Kinder werden geladen..." fullPage={false} />;
  }
  if (load.kind === "failed") {
    return (
      <EmptyState
        variant="compact"
        title="Die Kinder konnten nicht geladen werden."
        description="Bitte schließen Sie den Dialog und öffnen Sie ihn noch einmal."
      />
    );
  }
  if (visible.length === 0) {
    return <EmptyState variant="compact" {...emptyText(scope, hasSearch)} />;
  }
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm text-gray-600">
          {visible.length === 1 ? "1 Kind" : `${visible.length} Kinder`}
        </p>
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={onToggleAll}
          disabled={isAdding}
        >
          {allVisibleSelected ? "Auswahl aufheben" : "Alle auswählen"}
        </Button>
      </div>
      <ul className="grid gap-2">
        {visible.map((child) => (
          <li key={child.id}>
            <ChoiceTile
              selected={selected.has(child.id)}
              disabled={isAdding}
              tone="green"
            >
              <Checkbox
                checked={selected.has(child.id)}
                disabled={isAdding}
                onChange={() => onToggle(child.id)}
              />
              <span className="min-w-0 flex-1">
                <span className="block truncate font-medium text-gray-900">
                  {child.name}
                </span>
                <span className="block truncate text-gray-500">
                  {[child.detail, child.place].filter(Boolean).join(" · ") ||
                    "–"}
                </span>
              </span>
              <span className="shrink-0 text-gray-600 tabular-nums">
                {child.pickupTime ? `geht ${child.pickupTime}` : "ohne Gehzeit"}
              </span>
            </ChoiceTile>
          </li>
        ))}
      </ul>
    </div>
  );
}

function emptyText(
  scope: PresentChildScope,
  hasSearch: boolean,
): { title: string; description?: string } {
  if (hasSearch) {
    return { title: "Kein Kind mit diesem Namen gefunden." };
  }
  if (scope === "stays") {
    return {
      title: "Kein weiteres Kind bleibt noch länger.",
      description: "Unter „Alle anwesenden“ sehen Sie alle Kinder in der OGS.",
    };
  }
  if (scope === "offering") {
    return {
      title: "Kein weiteres anwesendes Kind hat dieses Angebot heute gebucht.",
      description: "Unter „Alle anwesenden“ sehen Sie alle Kinder in der OGS.",
    };
  }
  return { title: "Gerade ist kein weiteres Kind in der OGS." };
}
