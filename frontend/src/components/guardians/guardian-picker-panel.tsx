"use client";

import { useEffect, useState } from "react";
import { Search, ArrowLeft, Check, X } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  searchGuardians,
  GUARDIAN_PICKER_RESULT_LIMIT,
} from "@/lib/guardian-api";
import { getGuardianFullName, type Guardian } from "@/lib/guardian-helpers";
import {
  GuardianRoleSelect,
  RelationshipTypeSelect,
  RelationshipPermissionsFields,
  defaultGuardianRoleForRelationshipType,
  guardianRoleOperationalDefaults,
  type RelationshipFlag,
} from "~/components/guardians/guardian-relationship-fields";
import type { GuardianRole } from "~/lib/guardian-helpers";
import type { RelationshipFormData } from "~/components/guardians/guardian-form-modal";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GuardianPickerPanel" });

// Minimum characters before a search fires, and debounce delay. The minimum is
// also enforced server-side: it keeps the picker (open to all staff) from being
// used to enumerate the guardian pool one letter at a time (#1513).
const MIN_QUERY_LENGTH = 3;
const SEARCH_DEBOUNCE_MS = 300;

// Neutral, name-free label for how many other children a guardian is linked to.
// The picker never shows those children's names (they may belong to families the
// staff member doesn't supervise) — only the count, for disambiguation.
function linkedChildrenLabel(count: number): string {
  return count === 1
    ? "Erziehungsberechtigte/r von 1 weiteren Kind"
    : `Erziehungsberechtigte/r von ${count} weiteren Kindern`;
}

interface GuardianPickerPanelProps {
  // Called once the staff member has chosen an existing guardian and set the
  // relationship flags for THIS child. The picker never mutates the profile.
  readonly onSelect: (
    guardian: Guardian,
    relationship: RelationshipFormData,
  ) => void;
  // Collapses the panel without selecting anyone.
  readonly onCancel: () => void;
  // Guardian profile ids already added to / linked with this child. Shown
  // greyed-out so the same person can't be added twice.
  readonly excludeProfileIds?: readonly string[];
}

// Stable empty default so the prop keeps referential equality across renders.
const EMPTY_PROFILE_IDS: readonly string[] = [];

// Relationship defaults mirror createEmptyEntry() in the guardian form so the
// "new" and "existing" paths start from the same place.
const DEFAULT_RELATIONSHIP: RelationshipFormData = {
  relationshipType: "parent",
  guardianRole: "legal_guardian",
  isPrimary: false,
  isEmergencyContact: false,
  canPickup: false,
  emergencyPriority: 1,
};

// GuardianPickerPanel is the INLINE (non-modal) counterpart to "Neu anlegen".
// Linking an existing guardian is just a lookup, so it expands in place inside
// the host form/card instead of stacking a second modal — which also makes the
// two paths visually distinct. The host mounts it only while active, so every
// open starts from a fresh state.
export default function GuardianPickerPanel({
  onSelect,
  onCancel,
  excludeProfileIds = EMPTY_PROFILE_IDS,
}: GuardianPickerPanelProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Guardian[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Guardian | null>(null);
  const [relationship, setRelationship] =
    useState<RelationshipFormData>(DEFAULT_RELATIONSHIP);

  // Debounced search. Skips firing while a guardian is already selected (the
  // relationship step) or while the query is too short.
  useEffect(() => {
    if (selected) return;

    const trimmed = query.trim();
    if (trimmed.length < MIN_QUERY_LENGTH) {
      setResults([]);
      setLoading(false);
      setError(null);
      return;
    }

    setLoading(true);
    let cancelled = false;
    const handle = setTimeout(() => {
      searchGuardians(trimmed)
        .then((found) => {
          if (!cancelled) {
            setResults(found);
            setError(null);
          }
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            // Log the technical detail, but never surface a raw (English)
            // backend/JS message to the user — show a German message instead.
            logger.error("guardian_search_failed", {
              error: err instanceof Error ? err.message : String(err),
            });
            setResults([]);
            setError("Suche fehlgeschlagen. Bitte erneut versuchen.");
          }
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      cancelled = true;
      clearTimeout(handle);
    };
  }, [query, selected]);

  const updateRelationship = (field: RelationshipFlag, value: boolean) => {
    setRelationship((prev) => ({ ...prev, [field]: value }));
  };

  const updateGuardianRole = (role: GuardianRole) => {
    const defaults = guardianRoleOperationalDefaults(role);
    setRelationship((prev) => ({ ...prev, ...defaults, guardianRole: role }));
  };

  const updateRelationshipType = (relationshipType: string) => {
    const guardianRole =
      defaultGuardianRoleForRelationshipType(relationshipType);
    const defaults = guardianRoleOperationalDefaults(guardianRole);
    setRelationship((prev) => ({
      ...prev,
      ...defaults,
      relationshipType,
      guardianRole,
    }));
  };

  const handleConfirm = () => {
    if (!selected) return;
    onSelect(selected, relationship);
  };

  const excluded = new Set(excludeProfileIds);

  return (
    <div
      data-testid="guardian-picker-panel"
      className="border-moto-blue/40 bg-moto-blue-soft/40 space-y-3 rounded-xl border p-3 md:p-4"
    >
      {/* Panel header */}
      <div className="flex items-center justify-between">
        <h4 className="flex items-center gap-2 text-xs font-semibold text-gray-900 md:text-sm">
          <Search className="text-moto-blue-strong h-3.5 w-3.5 md:h-4 md:w-4" />
          Vorhandene/n suchen
        </h4>
        <button
          type="button"
          onClick={onCancel}
          aria-label="Suche schließen"
          className="rounded-lg p-1 text-gray-400 transition-colors hover:bg-white/60 hover:text-gray-600"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {selected ? (
        <div className="space-y-3">
          {/* Chosen person (read-only) */}
          <div className="rounded-lg border border-gray-200 bg-white p-3">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-gray-900">
                  {getGuardianFullName(selected)}
                </p>
                {selected.email && (
                  <p className="truncate text-xs text-gray-500">
                    {selected.email}
                  </p>
                )}
                {selected.linkedChildrenCount !== undefined &&
                  selected.linkedChildrenCount > 0 && (
                    <p className="mt-0.5 truncate text-xs text-gray-500">
                      {linkedChildrenLabel(selected.linkedChildrenCount)}
                    </p>
                  )}
              </div>
              <button
                type="button"
                onClick={() => setSelected(null)}
                className="flex flex-shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-white/60"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                Andere wählen
              </button>
            </div>
          </div>

          {/* Relationship for THIS child only (shared with the guardian form) */}
          <RelationshipTypeSelect
            id="picker-relationship-type"
            value={relationship.relationshipType}
            onChange={updateRelationshipType}
          />
          <GuardianRoleSelect
            id="picker-guardian-role"
            value={relationship.guardianRole}
            onChange={updateGuardianRole}
          />
          <RelationshipPermissionsFields
            isPrimary={relationship.isPrimary}
            canPickup={relationship.canPickup}
            isEmergencyContact={relationship.isEmergencyContact}
            onChange={updateRelationship}
          />

          <button
            type="button"
            onClick={handleConfirm}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-gray-700 md:text-sm"
          >
            <Check className="h-4 w-4" />
            Hinzufügen
          </button>
        </div>
      ) : (
        <>
          {/* Search input */}
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              // eslint-disable-next-line jsx-a11y/no-autofocus
              autoFocus
              placeholder="Name oder E-Mail suchen…"
              className="focus:border-moto-blue focus:ring-moto-blue block w-full rounded-lg border border-gray-200 bg-white py-2 pr-3 pl-9 text-sm transition-colors focus:ring-1"
            />
          </div>

          {error && (
            <div className="border-moto-red/20 bg-moto-red-soft rounded-lg border p-2 md:p-3">
              <p className="text-moto-red-strong text-xs md:text-sm">{error}</p>
            </div>
          )}

          {/* Results */}
          <div className="max-h-72 space-y-2 overflow-y-auto">
            {loading && (
              <p className="px-1 py-4 text-center text-xs text-gray-500">
                Suche läuft…
              </p>
            )}

            {!loading &&
              query.trim().length >= MIN_QUERY_LENGTH &&
              results.length === 0 &&
              !error && (
                <p className="px-1 py-4 text-center text-xs text-gray-500">
                  Keine Erziehungsberechtigten gefunden.
                </p>
              )}

            {!loading && query.trim().length < MIN_QUERY_LENGTH && (
              <p className="px-1 py-4 text-center text-xs text-gray-400">
                Mindestens {MIN_QUERY_LENGTH} Zeichen eingeben.
              </p>
            )}

            {!loading && results.length >= GUARDIAN_PICKER_RESULT_LIMIT && (
              <p className="px-1 pt-1 text-center text-xs text-gray-400">
                Viele Treffer – bitte den Namen weiter eingrenzen.
              </p>
            )}

            {results.map((guardian) => {
              const isExcluded = excluded.has(guardian.id);
              return (
                <button
                  key={guardian.id}
                  type="button"
                  disabled={isExcluded}
                  onClick={() => setSelected(guardian)}
                  className={`flex w-full items-start gap-2 rounded-lg border p-2 text-left transition-colors md:p-3 ${
                    isExcluded
                      ? "cursor-not-allowed border-gray-100 bg-gray-50 opacity-60"
                      : "hover:border-moto-blue hover:bg-moto-blue-soft/40 border-gray-100 bg-white"
                  }`}
                >
                  <MotoConceptIcon
                    concept="parents"
                    size={18}
                    className="mt-0.5"
                  />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-xs font-medium text-gray-900 md:text-sm">
                      {getGuardianFullName(guardian)}
                      {isExcluded && (
                        <span className="ml-2 font-normal text-gray-500">
                          (bereits hinzugefügt)
                        </span>
                      )}
                    </p>
                    {guardian.email && (
                      <p className="truncate text-xs text-gray-500">
                        {guardian.email}
                      </p>
                    )}
                    {guardian.linkedChildrenCount !== undefined &&
                      guardian.linkedChildrenCount > 0 && (
                        <p className="truncate text-xs text-gray-400">
                          {linkedChildrenLabel(guardian.linkedChildrenCount)}
                        </p>
                      )}
                  </div>
                </button>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}
