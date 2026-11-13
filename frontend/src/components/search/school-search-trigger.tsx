"use client";

import { Search } from "lucide-react";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";

import { Button } from "~/components/ui/button";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  SchoolSearchSlotContext,
  type SchoolSearchPlacement,
} from "./school-search-slot";

type SchoolSearchDialogComponent =
  typeof import("./school-search").SchoolSearchDialog;

// The dialog with its result rendering loads apart from the page, two seconds
// after the button mounts or on the first open. It renders only once its code
// is here: a lazy component (next/dynamic) suspends on its first render even
// with the code cached, and React then holds the reveal for 300 ms.
const loadedDialog: { component: SchoolSearchDialogComponent | null } = {
  component: null,
};
let dialogLoad: Promise<SchoolSearchDialogComponent> | null = null;

function loadSchoolSearchDialog(): Promise<SchoolSearchDialogComponent> {
  dialogLoad ??= import("./school-search").then(
    (mod) => (loadedDialog.component = mod.SchoolSearchDialog),
    (error: unknown) => {
      // The next open tries again.
      dialogLoad = null;
      throw error;
    },
  );
  return dialogLoad;
}

const PRELOAD_DELAY_MS = 2000;

const subscribeNothing = () => () => undefined;

/** "⌘ K" on a Mac, "Strg K" elsewhere; empty during server rendering. */
function useShortcutLabel(): string {
  return useSyncExternalStore(
    subscribeNothing,
    () => (/Mac|iPhone|iPad/.test(navigator.userAgent) ? "⌘ K" : "Strg K"),
    () => "",
  );
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    target.tagName === "INPUT" ||
    target.tagName === "TEXTAREA" ||
    target.tagName === "SELECT"
  );
}

interface SchoolSearchTriggerProps {
  /** From xl on the header shows a field-like button; see the slot. */
  readonly placement: SchoolSearchPlacement;
}

/**
 * The search button of the staff portal. On a wide screen it reads like a
 * search field with its shortcut; on a phone and tablet it is a magnifier.
 */
function SchoolSearchTrigger({
  placement,
}: Readonly<SchoolSearchTriggerProps>) {
  const [isOpen, setIsOpen] = useState(false);
  const [hasDialog, setHasDialog] = useState(loadedDialog.component !== null);
  const [loadFailed, setLoadFailed] = useState(false);
  const shortcut = useShortcutLabel();
  // The shortcut listener reads the open state without re-subscribing.
  const openRef = useRef(isOpen);
  useEffect(() => {
    openRef.current = isOpen;
  }, [isOpen]);

  useEffect(() => {
    // One listener for the page: only the header owns the shortcut.
    if (placement !== "header") return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key?.toLowerCase() !== "k") return;
      if (!(event.metaKey || event.ctrlKey)) return;
      if (event.altKey || event.shiftKey) return;
      // Inside a field Cmd/Ctrl+K stays the field's own (a link in an editor).
      if (isEditableTarget(event.target) && !openRef.current) return;
      event.preventDefault();
      setIsOpen((current) => !current);
    };
    globalThis.addEventListener("keydown", handleKeyDown);
    return () => globalThis.removeEventListener("keydown", handleKeyDown);
  }, [placement]);

  useEffect(() => {
    if (hasDialog) return;
    let live = true;
    const load = () =>
      loadSchoolSearchDialog().then(
        () => live && setHasDialog(true),
        () => {
          // A failed preload stays silent; an open the person asked for says so.
          if (live && openRef.current) {
            setIsOpen(false);
            setLoadFailed(true);
          }
        },
      );
    if (isOpen) void load();
    const preload = isOpen
      ? undefined
      : globalThis.setTimeout(() => void load(), PRELOAD_DELAY_MS);
    return () => {
      live = false;
      globalThis.clearTimeout(preload);
    };
  }, [hasDialog, isOpen]);

  const Dialog = hasDialog ? loadedDialog.component : null;

  return (
    <>
      {placement === "header" ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setIsOpen(true)}
          aria-keyshortcuts="Control+K Meta+K"
          aria-haspopup="dialog"
          className="hidden gap-2 xl:inline-flex"
        >
          <Search aria-hidden="true" className="h-4 w-4 text-gray-500" />
          <span className="text-gray-600">Überall suchen</span>
          {shortcut ? (
            <span className="ml-6 text-xs text-gray-400">{shortcut}</span>
          ) : null}
        </Button>
      ) : null}
      <Button
        type="button"
        variant="ghost"
        size="icon"
        onClick={() => setIsOpen(true)}
        aria-label="Überall suchen"
        aria-keyshortcuts="Control+K Meta+K"
        aria-haspopup="dialog"
        className={placement === "header" ? "xl:hidden" : "lg:hidden"}
      >
        <Search aria-hidden="true" className="h-[18px] w-[18px]" />
      </Button>
      {isOpen && Dialog ? (
        <Dialog isOpen onClose={() => setIsOpen(false)} />
      ) : null}
      {loadFailed ? (
        <SearchLoadFailed onShown={() => setLoadFailed(false)} />
      ) : null}
    </>
  );
}

/** Says once that the search could not be opened (no network, new release). */
function SearchLoadFailed({ onShown }: Readonly<{ onShown: () => void }>) {
  const { showMessage } = useApiErrorDisplay();
  useEffect(() => {
    showMessage(
      "Die Suche konnte nicht geöffnet werden. Bitte versuchen Sie es noch einmal.",
    );
    onShown();
  }, [showMessage, onShown]);
  return null;
}

function renderSchoolSearchTrigger(placement: SchoolSearchPlacement) {
  return <SchoolSearchTrigger placement={placement} />;
}

/**
 * Puts the search into the shared header and page head of the staff portal.
 * Only the staff layout renders it, so no other portal loads the search.
 */
export function SchoolSearchProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <SchoolSearchSlotContext.Provider value={renderSchoolSearchTrigger}>
      {children}
    </SchoolSearchSlotContext.Provider>
  );
}
