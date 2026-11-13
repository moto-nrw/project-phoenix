"use client";

import dynamic from "next/dynamic";
import { Search } from "lucide-react";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";

import { Button } from "~/components/ui/button";

// The dialog with its result rendering loads on first use, so every page
// carries only this button and the shortcut.
const SchoolSearchDialog = dynamic(
  () => import("./school-search").then((mod) => mod.SchoolSearchDialog),
  { ssr: false },
);

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
  /**
   * `header`: the staff header from lg on, a field-like button from xl on and
   * the owner of the shortcut Cmd/Ctrl+K. `pageHead`: the magnifier in the
   * page head below lg, where the shell has no header (phone, tablet).
   */
  readonly placement: "header" | "pageHead";
}

/**
 * The search button of the staff portal. On a wide screen it reads like a
 * search field with its shortcut; on a phone and tablet it is a magnifier.
 */
export function SchoolSearchTrigger({
  placement,
}: Readonly<SchoolSearchTriggerProps>) {
  const [isOpen, setIsOpen] = useState(false);
  const shortcut = useShortcutLabel();
  const openRef = useRef(isOpen);
  openRef.current = isOpen;

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
      {isOpen ? (
        <SchoolSearchDialog isOpen onClose={() => setIsOpen(false)} />
      ) : null}
    </>
  );
}
