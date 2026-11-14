"use client";

import { Search } from "lucide-react";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

import { Button } from "~/components/ui/button";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import type { SchoolSearchScope } from "~/lib/school-search";
import {
  SchoolSearchSlotContext,
  type SchoolSearchPlacement,
  type SchoolSearchSlotOptions,
} from "./school-search-slot";

type SchoolSearchDialogComponent =
  typeof import("./school-search").SchoolSearchDialog;

// The dialog with its result rendering loads apart from the page, two seconds
// after the staff layout mounts or on the first open. It renders only once its
// code is here: a lazy component (next/dynamic) suspends on its first render
// even with the code cached, and React then holds the reveal for 300 ms.
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

interface SchoolSearchButtonProps {
  readonly placement: SchoolSearchPlacement;
  readonly onOpen: () => void;
}

/**
 * The search button of the staff portal. In the wide header and in the
 * phone's "Mehr" sheet it reads like a search field; in the tablet's page
 * head and the narrower header it is a magnifier.
 */
function SchoolSearchButton({
  placement,
  onOpen,
}: Readonly<SchoolSearchButtonProps>) {
  const shortcut = useShortcutLabel();
  if (placement === "moreSheet") {
    return (
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onOpen}
        aria-haspopup="dialog"
        className="w-full justify-start gap-3"
      >
        <Search aria-hidden="true" className="h-4 w-4 text-gray-500" />
        <span className="text-gray-600">Überall suchen</span>
      </Button>
    );
  }
  return (
    <>
      {placement === "header" ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={onOpen}
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
        onClick={onOpen}
        aria-label="Überall suchen"
        aria-keyshortcuts="Control+K Meta+K"
        aria-haspopup="dialog"
        // Below sm the phone's "Mehr" sheet carries the search: the page
        // head keeps its width for the title and the page's own actions.
        className={
          placement === "header"
            ? "xl:hidden"
            : "hidden sm:inline-flex lg:hidden"
        }
      >
        <Search aria-hidden="true" className="h-[18px] w-[18px]" />
      </Button>
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

/**
 * Puts the search into the shared header and page head of the staff portal
 * and owns the one search dialog with its shortcut Cmd/Ctrl+K, on every
 * width. Only the staff layout renders it, so no other portal loads the
 * search.
 */
export function SchoolSearchProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const [isOpen, setIsOpen] = useState(false);
  const [hasDialog, setHasDialog] = useState(loadedDialog.component !== null);
  const [loadFailed, setLoadFailed] = useState(false);
  // What the server granted this account, from its last answer.
  const [scope, setScope] = useState<SchoolSearchScope | null>(null);
  // The shortcut listener reads the open state without re-subscribing.
  const openRef = useRef(isOpen);
  useEffect(() => {
    openRef.current = isOpen;
  }, [isOpen]);

  useEffect(() => {
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
  }, []);

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

  // Where the focus goes back when the opener is gone by the time the search
  // closes (the "Mehr" sheet closes as the search opens). It moves once the
  // dialog has unmounted, after the dialog's own focus return, which finds no
  // opener here.
  const returnFocusRef =
    useRef<SchoolSearchSlotOptions["returnFocus"]>(undefined);
  useEffect(() => {
    const target = returnFocusRef.current;
    if (isOpen || !target) return;
    returnFocusRef.current = undefined;
    const timer = globalThis.setTimeout(() => target()?.focus(), 0);
    return () => globalThis.clearTimeout(timer);
  }, [isOpen]);
  const close = useCallback(() => setIsOpen(false), []);
  const clearLoadFailed = useCallback(() => setLoadFailed(false), []);
  const renderButton = useCallback(
    (placement: SchoolSearchPlacement, options: SchoolSearchSlotOptions) => (
      <SchoolSearchButton
        placement={placement}
        onOpen={() => {
          options.onOpen?.();
          returnFocusRef.current = options.returnFocus;
          setIsOpen(true);
        }}
      />
    ),
    [],
  );

  const Dialog = hasDialog ? loadedDialog.component : null;
  return (
    <SchoolSearchSlotContext.Provider value={renderButton}>
      {children}
      {isOpen && Dialog ? (
        <Dialog isOpen onClose={close} scope={scope} onScope={setScope} />
      ) : null}
      {loadFailed ? <SearchLoadFailed onShown={clearLoadFailed} /> : null}
    </SchoolSearchSlotContext.Provider>
  );
}
