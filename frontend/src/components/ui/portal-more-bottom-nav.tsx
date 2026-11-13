"use client";

import type { ComponentProps, ReactNode } from "react";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import {
  PortalBottomNav,
  PortalBottomNavButton,
} from "~/components/ui/portal-bottom-nav";

/**
 * Das Feld „Mehr“: Name, Bild (Konzept oder eigenes Icon), Zähler und die
 * Klassen der übrigen Felder. Zustand und Öffnen regelt die Leiste.
 */
export type PortalMoreButtonProps = WithoutOpenState<
  ComponentProps<typeof PortalBottomNavButton>
> & { readonly [attribute: `data-${string}`]: string | undefined };

// Je Bild-Variante (Konzept oder eigenes Icon) einzeln, sonst verliert `Omit`
// die Unterscheidung.
type WithoutOpenState<T> = T extends unknown
  ? Omit<T, "active" | "onClick" | "aria-haspopup" | "aria-expanded">
  : never;

/**
 * Die mobile Leiste mit „Mehr“ (#6580), geteilt von Tenant-, Eltern- und
 * Träger-Portal und den Einstellungen: die Felder der Leiste, dahinter das
 * Feld „Mehr“, das ein Blatt von unten öffnet. Was im Blatt steht, liefert
 * der Aufrufer.
 *
 * „Mehr“ leuchtet, solange das Blatt offen ist oder eine seiner Seiten die
 * aktuelle ist; sonst stünde man auf einer Seite vor einer Leiste, in der
 * nichts markiert ist.
 *
 * Der Aufrufer hält den Offen-Zustand, weil Zeilen im Blatt es schließen und
 * die gleitende Fläche der Leiste (`indicator`) neu messen muss, wenn „Mehr“
 * beim Öffnen seinen Namen zeigt.
 */
export function PortalMoreBottomNav({
  children,
  open,
  onOpenChange,
  moreItemActive,
  more,
  sheetTitle,
  sheetDescription,
  sheetMounted = true,
  sheet,
  ...barProps
}: Omit<ComponentProps<typeof PortalBottomNav>, "children"> & {
  /** Die Felder vor „Mehr“ (`PortalBottomNavLink`). */
  readonly children: ReactNode;
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  /** Eine Seite hinter „Mehr“ ist die aktuelle. */
  readonly moreItemActive: boolean;
  readonly more: PortalMoreButtonProps;
  /** Name und Beschreibung des Blatts für Vorleseprogramme. */
  readonly sheetTitle: string;
  readonly sheetDescription: string;
  /**
   * `false` nimmt das Blatt aus dem Baum, solange ein Dialog offen ist, den
   * eine Zeile des Blatts geöffnet hat (Abmelden in der Eltern-App).
   */
  readonly sheetMounted?: boolean;
  /** Der Inhalt des Blatts. */
  readonly sheet: ReactNode;
}) {
  return (
    <>
      <PortalBottomNav {...barProps}>
        {children}
        <PortalBottomNavButton
          {...more}
          active={open || moreItemActive}
          onClick={() => onOpenChange(true)}
          aria-haspopup="dialog"
          aria-expanded={open}
        />
      </PortalBottomNav>

      {sheetMounted ? (
        <Drawer open={open} onOpenChange={onOpenChange}>
          <DrawerContent className="bg-white">
            <DrawerHeader className="sr-only">
              <DrawerTitle>{sheetTitle}</DrawerTitle>
              <DrawerDescription>{sheetDescription}</DrawerDescription>
            </DrawerHeader>
            {sheet}
          </DrawerContent>
        </Drawer>
      ) : null}
    </>
  );
}
