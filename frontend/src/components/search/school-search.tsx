"use client";

import { useSession } from "next-auth/react";
import { useCallback, useEffect, useMemo, useState } from "react";

import {
  CommandPalette,
  CommandPaletteHighlight,
  type CommandPaletteGroup,
  type CommandPaletteItem,
} from "~/components/ui/command-palette";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Skeleton } from "~/components/ui/skeleton";
import {
  STAFF_NAV_ITEMS,
  staffNavItemPermitted,
  staffNavSettingOn,
  useStaffNavSettings,
  type StaffNavItem,
} from "~/components/dashboard/staff-nav-items";
import { hasPermission, hasRole, isCaregiver } from "~/lib/auth-utils";
import { childrenConcept } from "~/lib/concepts/children";
import { groupsConcept } from "~/lib/concepts/groups";
import { parentsConcept } from "~/lib/concepts/parents";
import { roomsConcept } from "~/lib/concepts/rooms";
import { staffConcept } from "~/lib/concepts/staff";
import { getRelationshipTypeLabel } from "~/lib/guardian-helpers";
import { useModuleNavigation } from "~/lib/hooks/use-module-navigation";
import { schoolClassLabel } from "~/lib/school-class-label";
import {
  answerQuestionLabel,
  looksLikePhoneNumber,
  searchSchool,
  SCHOOL_SEARCH_MAX_LENGTH,
  SCHOOL_SEARCH_MIN_LENGTH,
  type SchoolSearchChild,
  type SchoolSearchContact,
  type SchoolSearchResult,
} from "~/lib/school-search";
import { foldForSearch } from "~/lib/search-fold";
import { useOpenCareGroupMode, usePresenceMode } from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import { useApiLoadError } from "~/contexts/ToastContext";

/** Waiting time after the last key before the search asks the server. */
const SEARCH_DEBOUNCE_MS = 200;

const EMPTY_RESULT: SchoolSearchResult = {
  children: [],
  contacts: [],
  staff: [],
  rooms: [],
  groups: [],
};

type SearchState =
  | { readonly kind: "idle" }
  | { readonly kind: "loading"; readonly term: string }
  | { readonly kind: "ready"; readonly term: string }
  | { readonly kind: "error"; readonly term: string };

/**
 * Runs the search for the typed text, debounced. The last answer stays
 * visible while the next one loads, so the list never flashes empty. An
 * answer for an older text is dropped. A failure goes the shared load error
 * path; neither it nor anything else here logs the text.
 */
function useSchoolSearch(query: string, enabled: boolean) {
  const [state, setState] = useState<SearchState>({ kind: "idle" });
  const [lastResult, setLastResult] = useState<SchoolSearchResult | null>(null);
  const [attempt, setAttempt] = useState(0);
  const {
    error,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const term = query.trim();
  const searchable = enabled && term.length >= SCHOOL_SEARCH_MIN_LENGTH;

  useEffect(() => {
    clearLoadError();
    if (!searchable) {
      setState((current) =>
        current.kind === "idle" ? current : { kind: "idle" },
      );
      setLastResult(null);
      return;
    }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      setState({ kind: "loading", term });
      searchSchool(term, controller.signal)
        .then((result) => {
          if (controller.signal.aborted) return;
          setState({ kind: "ready", term });
          setLastResult(result);
        })
        .catch((failure: unknown) => {
          // An aborted request was replaced by a newer text; it has no
          // result to show.
          if (controller.signal.aborted) return;
          setState({ kind: "error", term });
          setLastResult(null);
          void showLoadError(failure, {
            object: "die Suche",
            retry: () => setAttempt((count) => count + 1),
          });
        });
    }, SEARCH_DEBOUNCE_MS);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [searchable, term, attempt, showLoadError, clearLoadError]);

  const settled =
    (state.kind === "ready" || state.kind === "error") && state.term === term;
  return {
    result: searchable ? lastResult : null,
    busy: searchable && !settled,
    error: searchable ? error : null,
  };
}

/** The flat pages of the staff navigation this person may open. */
function useSearchablePages(): readonly StaffNavItem[] {
  const { data: session } = useSession();
  const settings = useStaffNavSettings();
  const moduleNav = useModuleNavigation();
  const openCareGroupMode = useOpenCareGroupMode();
  const isBinaryMode = usePresenceMode() === "binary";
  return useMemo(() => {
    const viewer = {
      session,
      isAdmin: hasRole(session, "admin"),
      isCaregiver: isCaregiver(session),
      openCareGroupMode,
    };
    return STAFF_NAV_ITEMS.filter(
      (item) =>
        !(isBinaryMode && item.binaryHidden) &&
        moduleNav.page(item.href) === "open" &&
        staffNavSettingOn(item, settings) &&
        staffNavItemPermitted(item, viewer),
    );
  }, [session, settings, moduleNav, openCareGroupMode, isBinaryMode]);
}

function childSubtitle(child: SchoolSearchChild): string {
  const parts = [
    child.schoolClass ? schoolClassLabel(child.schoolClass) : "",
    child.groupName ? `Gruppe ${child.groupName}` : "",
  ].filter(Boolean);
  const { match } = child;
  if (match.field === "parent_note" && match.text) {
    parts.push(`Elternnotiz: ${match.text}`);
  }
  if (match.field === "enrollment_answer") {
    // "Mein Kind darf schwimmen." / "Seepferdchen?" lose their own end mark
    // before the answer follows.
    const question = answerQuestionLabel(match.label ?? "");
    parts.push(`Anmeldung: ${question}: ${match.text ?? ""}`);
  }
  return parts.join(" · ");
}

function contactItems(
  contacts: readonly SchoolSearchContact[],
  query: string,
  open: (href: string) => void,
): CommandPaletteItem[] {
  return contacts.flatMap((contact) => {
    const name = `${contact.firstName} ${contact.lastName}`;
    const reached =
      contact.match.field === "phone" || contact.match.field === "email"
        ? (contact.match.text ?? "")
        : "";
    return contact.children.map((child) => {
      const childName = `${child.firstName} ${child.lastName}`;
      const relation = `${getRelationshipTypeLabel(child.relationshipType)} von ${childName}`;
      const family = child.schoolClass
        ? `${relation} (${schoolClassLabel(child.schoolClass)})`
        : relation;
      return {
        id: `contact-${contact.id}-${child.id}`,
        label: reached
          ? `${name}, ${reached}, ${family}`
          : `${name}, ${family}`,
        title: <CommandPaletteHighlight text={name} query={query} />,
        subtitle: reached ? (
          <>
            <CommandPaletteHighlight text={reached} query={query} />
            {` · ${family}`}
          </>
        ) : (
          family
        ),
        leading: <ConceptIconTile concept={parentsConcept} size="xs" />,
        onSelect: () =>
          open(
            `/students/${encodeURIComponent(child.id)}?tab=erziehungsberechtigte`,
          ),
      };
    });
  });
}

function buildGroups(
  result: SchoolSearchResult,
  pages: readonly StaffNavItem[],
  query: string,
  open: (href: string, newTab?: boolean) => void,
): CommandPaletteGroup[] {
  const folded = foldForSearch(query.trim());
  const matchingPages = folded
    ? pages.filter((page) => foldForSearch(page.label).includes(folded))
    : pages;
  return [
    {
      id: "children",
      title: "Kinder",
      items: result.children.map((child) => {
        const name = `${child.firstName} ${child.lastName}`;
        const subtitle = childSubtitle(child);
        return {
          id: `child-${child.id}`,
          label: subtitle ? `${name}, ${subtitle}` : name,
          title: <CommandPaletteHighlight text={name} query={query} />,
          subtitle: subtitle ? (
            <CommandPaletteHighlight text={subtitle} query={query} />
          ) : undefined,
          leading: <ConceptIconTile concept={childrenConcept} size="xs" />,
          onSelect: () => open(`/students/${encodeURIComponent(child.id)}`),
        };
      }),
    },
    {
      id: "contacts",
      title: "Sorgeberechtigte und Kontakte",
      items: contactItems(result.contacts, query, open),
    },
    {
      id: "staff",
      title: "Mitarbeitende",
      items: result.staff.map((member) => {
        const name = `${member.firstName} ${member.lastName}`;
        return {
          id: `staff-${member.id}`,
          label: name,
          title: <CommandPaletteHighlight text={name} query={query} />,
          leading: <ConceptIconTile concept={staffConcept} size="xs" />,
          onSelect: () => open(`/staff/${encodeURIComponent(member.id)}`),
        };
      }),
    },
    {
      id: "rooms",
      title: "Räume",
      items: result.rooms.map((room) => ({
        id: `room-${room.id}`,
        label: room.building ? `${room.name}, ${room.building}` : room.name,
        title: <CommandPaletteHighlight text={room.name} query={query} />,
        subtitle: room.building,
        leading: <ConceptIconTile concept={roomsConcept} size="xs" />,
        onSelect: () => open(`/rooms/${encodeURIComponent(room.id)}`),
      })),
    },
    {
      id: "groups",
      title: "Gruppen",
      items: result.groups.map((group) => ({
        id: `group-${group.id}`,
        label: `Gruppe ${group.name}, Kinder anzeigen`,
        title: <CommandPaletteHighlight text={group.name} query={query} />,
        subtitle: "Kinder der Gruppe anzeigen",
        leading: <ConceptIconTile concept={groupsConcept} size="xs" />,
        onSelect: () =>
          open(`/students/search?group_id=${encodeURIComponent(group.id)}`),
      })),
    },
    {
      id: "pages",
      title: "Seiten",
      items: matchingPages.map((page) => ({
        id: `page-${page.href}`,
        label: page.label,
        title: <CommandPaletteHighlight text={page.label} query={query} />,
        leading: <ConceptIconTile concept={page.concept} size="xs" />,
        onSelect: () => open(page.href, page.newTab),
      })),
    },
  ];
}

function countHits(groups: readonly CommandPaletteGroup[]): number {
  return groups.reduce((sum, group) => sum + group.items.length, 0);
}

function SearchSkeleton() {
  return (
    <div aria-hidden="true" className="space-y-3 px-3 pt-3">
      {[0, 1, 2, 3].map((row) => (
        <div key={row} className="flex items-center gap-3">
          <Skeleton shape="control" className="h-8 w-8" />
          <div className="flex-1 space-y-1.5">
            <Skeleton className="h-3.5 w-2/5" />
            <Skeleton className="h-3 w-3/5" />
          </div>
        </div>
      ))}
    </div>
  );
}

interface SchoolSearchDialogProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}

/**
 * The search over the school (#6954): children, contact persons, staff,
 * rooms, groups and the pages of the app, in one dialog. What a person may
 * find is decided by the server; this dialog only explains a phone search
 * that the account cannot run.
 */
export function SchoolSearchDialog({
  isOpen,
  onClose,
}: Readonly<SchoolSearchDialogProps>) {
  const router = useTenantRouter();
  const { data: session } = useSession();
  const pages = useSearchablePages();
  const [query, setQuery] = useState("");
  const { result, busy, error } = useSchoolSearch(query, isOpen);
  const term = query.trim();
  const isPhone = looksLikePhoneNumber(term);
  const readsContacts =
    hasRole(session, "admin") || hasPermission(session, "users:read");

  const close = useCallback(() => {
    setQuery("");
    onClose();
  }, [onClose]);

  const open = useCallback(
    (href: string, newTab?: boolean) => {
      close();
      if (newTab) {
        globalThis.open(href, "_blank", "noopener");
        return;
      }
      router.push(href);
    },
    [close, router],
  );

  const groups = useMemo(
    () => buildGroups(result ?? EMPTY_RESULT, pages, term, open),
    [result, pages, term, open],
  );
  const hits = countHits(groups);
  const searching = term.length >= SCHOOL_SEARCH_MIN_LENGTH;

  let placeholderContent: React.ReactNode = null;
  if (error) {
    placeholderContent = (
      <div className="px-3 pt-3">
        <LoadErrorAlert error={error} actionLayout="responsive" />
      </div>
    );
  } else if (busy) {
    placeholderContent = <SearchSkeleton />;
  } else if (searching) {
    placeholderContent = (
      <EmptyState
        variant="compact"
        title="Nichts gefunden"
        description={
          isPhone && !readsContacts
            ? "Telefonnummern finden nur Konten, die Kontaktdaten sehen dürfen."
            : "Suchen Sie nach Name, Klasse, Gruppe, Telefonnummer oder E-Mail."
        }
      />
    );
  }

  // A hint, never a link carrying the text: the typed text stays out of
  // every URL (ADR 0227).
  const footer = (
    <span className="text-gray-500">
      {readsContacts
        ? "Auch Telefonnummern und Anmeldeangaben"
        : "Kinder, Räume, Gruppen und Seiten"}
    </span>
  );

  return (
    <CommandPalette
      isOpen={isOpen}
      onClose={close}
      label="Überall suchen"
      query={query}
      onQueryChange={setQuery}
      placeholder={
        readsContacts
          ? "Name, Telefonnummer, Klasse …"
          : "Name, Klasse, Seite …"
      }
      groups={groups}
      busy={busy && hits > 0}
      placeholderContent={placeholderContent}
      footer={footer}
      status={searching && !busy && !error ? `${hits} Treffer` : undefined}
      maxLength={SCHOOL_SEARCH_MAX_LENGTH}
    />
  );
}
