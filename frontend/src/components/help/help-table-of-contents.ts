import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type MouseEvent,
} from "react";

const SECTION_ACTIVATION_OFFSET = 160;

export interface HelpTableOfContentsItem {
  readonly id: string;
  readonly children?: readonly { readonly id: string }[];
}

function sectionIdsFrom(
  items: readonly HelpTableOfContentsItem[],
): readonly string[] {
  return items.flatMap((item) => [
    item.id,
    ...(item.children?.map((child) => child.id) ?? []),
  ]);
}

/**
 * Hält die rechte Artikelnavigation beim Scrollen auf dem sichtbaren
 * Abschnitt und scrollt nach einem Klick ruhig zum Ziel.
 */
export function useHelpTableOfContents(
  items: readonly HelpTableOfContentsItem[],
) {
  const sectionKey = sectionIdsFrom(items).join("\u0000");
  const sectionIds = useMemo(
    () => (sectionKey ? sectionKey.split("\u0000") : []),
    [sectionKey],
  );
  const [activeSectionId, setActiveSectionId] = useState<string | undefined>(
    sectionIds[0],
  );

  useEffect(() => {
    if (sectionIds.length === 0) {
      setActiveSectionId(undefined);
      return;
    }

    let animationFrame = 0;
    const updateActiveSection = () => {
      let nextSectionId = sectionIds[0];
      for (const sectionId of sectionIds) {
        const section = document.getElementById(sectionId);
        if (!section) continue;
        if (section.getBoundingClientRect().top <= SECTION_ACTIVATION_OFFSET) {
          nextSectionId = sectionId;
        } else {
          break;
        }
      }

      const pageHeight = document.documentElement.scrollHeight;
      const reachedPageEnd =
        pageHeight > window.innerHeight &&
        window.scrollY + window.innerHeight >= pageHeight - 2;
      if (reachedPageEnd) nextSectionId = sectionIds.at(-1);

      setActiveSectionId((current) =>
        current === nextSectionId ? current : nextSectionId,
      );
    };
    const scheduleUpdate = () => {
      cancelAnimationFrame(animationFrame);
      animationFrame = requestAnimationFrame(updateActiveSection);
    };

    updateActiveSection();
    window.addEventListener("scroll", scheduleUpdate, { passive: true });
    window.addEventListener("resize", scheduleUpdate);
    return () => {
      cancelAnimationFrame(animationFrame);
      window.removeEventListener("scroll", scheduleUpdate);
      window.removeEventListener("resize", scheduleUpdate);
    };
  }, [sectionIds]);

  const scrollToSection = useCallback(
    (event: MouseEvent<HTMLAnchorElement>, sectionId: string) => {
      const section = document.getElementById(sectionId);
      if (!section) return;

      event.preventDefault();
      const reduceMotion = window.matchMedia(
        "(prefers-reduced-motion: reduce)",
      ).matches;
      section.scrollIntoView({
        behavior: reduceMotion ? "auto" : "smooth",
        block: "start",
      });
      window.history.replaceState(window.history.state, "", `#${sectionId}`);
      setActiveSectionId(sectionId);
    },
    [],
  );

  return { activeSectionId, scrollToSection } as const;
}
