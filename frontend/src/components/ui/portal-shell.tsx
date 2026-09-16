import type { ReactNode } from "react";
import {
  NavigationProgressBar,
  NavigationProgressProvider,
} from "~/components/ui/navigation-progress";

/**
 * Das gemeinsame Gerüst aller vier Portale: gepunkteter Hintergrund,
 * klebende Kopfzeile, Seitennavigation neben dem Inhalt, mobile Leiste
 * darunter.
 *
 * Jedes Portal hält seine eigene Navigation (`AppShell`, `ParentShell`,
 * `SchoolShell`) — die Navigationsziele eines Portals gehören zu diesem
 * Portal. Der Rahmen darum ist überall derselbe und steht deshalb genau
 * einmal hier, statt in jeder Hülle erneut ausgeschrieben zu werden.
 *
 * Was sich zwischen den Portalen unterscheidet, kommt als Slot herein:
 * `topLayer` liegt über dem Hintergrund und vor der Kopfzeile (mobiler
 * Deckstreifen im Personal-Portal, sichere Fläche oben in der Eltern-App),
 * `headerClassName` bestimmt Ebene und Sichtbarkeit der Kopfzeile.
 */
interface PortalShellProps {
  readonly header: ReactNode;
  readonly headerClassName?: string;
  readonly backgroundClassName?: string;
  /**
   * Klassen der Inhaltshülle direkt um `children`. Das Personal-Portal gibt
   * hier eine Flex-Spalte hinein (`flex flex-1 flex-col`), damit eine Seite
   * die Höhe des Bildschirms bis zur Unterkante füllen kann; ohne Angabe
   * bleibt die Hülle ein gewöhnlicher Block.
   */
  readonly contentClassName?: string;
  readonly topLayer?: ReactNode;
  readonly sidebar: ReactNode;
  readonly bottomNav: ReactNode;
  readonly children: ReactNode;
}

export function PortalShell({
  header,
  headerClassName = "sticky top-0 z-40",
  backgroundClassName = "",
  contentClassName = "",
  topLayer,
  sidebar,
  bottomNav,
  children,
}: PortalShellProps) {
  return (
    <NavigationProgressProvider>
      {/*
       * Eine Flex-Spalte von der Wurzel bis zur Inhaltshülle: so reicht die
       * Zeile aus Seitenleiste und <main> immer bis zur Unterkante des
       * Bildschirms, und eine Seite kann ihren Inhalt daran ausrichten (die
       * letzte Fläche des Personal-Gerüsts wächst bis dorthin). Jede Stufe hat
       * genau ein Kind, deshalb ändert die Spalte an Abständen nichts.
       */}
      <div className="relative flex min-h-screen flex-col">
        <NavigationProgressBar />
        <div
          data-portal-background
          className={`moto-dotted-background moto-dotted-background--app-fixed moto-dotted-background--fullscreen pointer-events-none z-0 ${backgroundClassName}`}
          aria-hidden="true"
        />
        {topLayer}

        <div className={headerClassName}>{header}</div>

        <div className="relative z-10 flex flex-1">
          {sidebar}

          {/*
           * Der Seitenrand in drei Stufen, und jede Stufe steht genau einmal
           * da (#2824): 32 px auf dem grossen Monitor, 20 px auf einem
           * kleinen oder flachen Bildschirm (`compact`), 16 px unter 768 px.
           * Der Desktop-Wert ist bewusst der unvariierte Grundwert — sonst
           * stuenden zwei Varianten fuer dasselbe Padding nebeneinander.
           * Die 7rem unten halten die mobile Leiste frei; sie gibt es nur
           * unter `lg`, deshalb traegt die flache Desktop-Stufe sie nicht.
           */}
          <main className="compact:p-5 compact:max-lg:pb-[calc(7rem+env(safe-area-inset-bottom))] flex min-w-0 flex-1 flex-col p-8 max-md:p-4 max-md:pb-[calc(7rem+env(safe-area-inset-bottom))]">
            <div className={`relative z-10 ${contentClassName}`}>
              {children}
            </div>
          </main>
        </div>

        {bottomNav}
      </div>
    </NavigationProgressProvider>
  );
}
