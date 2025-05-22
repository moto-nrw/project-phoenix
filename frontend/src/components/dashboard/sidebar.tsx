// components/dashboard/sidebar.tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { HelpButton } from "@/components/ui/help_button";
import { UserContextProvider, useHasEducationalGroups } from "~/lib/usercontext-context";
import type {ReactNode} from "react";

// Type für Navigation Items
interface NavItem {
    href: string;
    label: string;
    icon: string;
    helpContent?: ReactNode;
}

// Additional Help Content for Specific Pages
const SPECIFIC_PAGE_HELP: Record<string, ReactNode> = {
    // Student Detail Page Help Content
    "student-detail": (
        <div>
            <p>Im <strong>Schülerprofil</strong> finden Sie:</p>
            <ul className="mt-3 space-y-2">
                <li>• <strong>Persönliche Daten:</strong> Name, Klasse, Gruppe und Geburtsdatum</li>
                <li>• <strong>Aktueller Standort:</strong> Wo sich der Schüler gerade befindet</li>
                <li>• <strong>Erziehungsberechtigte:</strong> Kontaktdaten für Notfälle</li>
            </ul>
            <p className="mt-4"><strong>Verfügbare Funktionen:</strong></p>
            <ul className="mt-2 space-y-1 text-sm">
                <li>• <strong>Raumverlauf:</strong> Chronologische Übersicht der besuchten Räume</li>
                <li>• <strong>Feedbackhistorie:</strong> Pädagogische Rückmeldungen und Notizen</li>
                <li>• <strong>Mensaverlauf:</strong> Teilnahme am Mittagessen</li>
                <li>• <strong>Schnellkontakt:</strong> Direkter Kontakt zu Erziehungsberechtigten</li>
            </ul>
        </div>
    ),
    // Room History Page Help Content
    "room_history": (
        <div>
            <p>Im <strong>Raumverlauf</strong> können Sie:</p>
            <ul className="mt-3 space-y-2">
                <li>• <strong>Chronologisch verfolgen:</strong> Welche Räume ein Schüler wann besucht hat</li>
                <li>• <strong>Verweildauer einsehen:</strong> Wie lange sich ein Schüler in jedem Raum aufgehalten hat</li>
                <li>• <strong>Nach Zeiträumen filtern:</strong> Heute, Diese Woche, Letzte 7 Tage oder Diesen Monat</li>
            </ul>
            <p className="mt-4"><strong>Anwendungsmöglichkeiten:</strong></p>
            <ul className="mt-2 space-y-1 text-sm">
                <li>• <strong>Anwesenheitskontrolle:</strong> Überprüfen, ob ein Schüler an bestimmten Aktivitäten teilgenommen hat</li>
                <li>• <strong>Verhaltensanalyse:</strong> Raumbewegungen eines Schülers nachvollziehen</li>
                <li>• <strong>Konfliktsituationen klären:</strong> Bei Unstimmigkeiten den tatsächlichen Aufenthaltsort ermitteln</li>
                <li>• <strong>Abholsituation prüfen:</strong> Wann ein Schüler die Einrichtung verlassen hat</li>
            </ul>
        </div>
    ),
    // Feedback History Page Help Content
    "feedback_history": (
        <div>
            <p>In der <strong>Feedbackhistorie</strong> finden Sie:</p>
            <ul className="mt-3 space-y-2">
                <li>• <strong>Pädagogische Rückmeldungen:</strong> Wichtige Beobachtungen zum Schüler</li>
                <li>• <strong>Verhaltensnotizen:</strong> Dokumentierte Verhaltensauffälligkeiten und positive Entwicklungen</li>
                <li>• <strong>Elterngespräche:</strong> Protokolle zu Gesprächen mit Erziehungsberechtigten</li>
            </ul>
            <p className="mt-4"><strong>Funktionen und Möglichkeiten:</strong></p>
            <ul className="mt-2 space-y-1 text-sm">
                <li>• <strong>Chronologische Sortierung:</strong> Feedback-Einträge nach Datum geordnet</li>
                <li>• <strong>Kategorisierung:</strong> Farbliche Kennzeichnung nach Art des Feedbacks</li>
                <li>• <strong>Filtermöglichkeiten:</strong> Nach Zeitraum, Kategorie oder Verfasser filtern</li>
                <li>• <strong>Neues Feedback hinzufügen:</strong> Direkt neue Beobachtungen dokumentieren</li>
                <li>• <strong>Exportfunktion:</strong> Feedbackhistorie für Elterngespräche ausdrucken</li>
            </ul>
        </div>
    ),
    // Mensa History Page Help Content
    "mensa_history": (
        <div>
            <p>Im <strong>Mensaverlauf</strong> können Sie einsehen:</p>
            <ul className="mt-3 space-y-2">
                <li>• <strong>Mahlzeitenteilnahme:</strong> An welchen Tagen der Schüler am Mittagessen teilgenommen hat</li>
                <li>• <strong>Essensauswahl:</strong> Welche Menüs ausgewählt wurden (wenn angeboten)</li>
                <li>• <strong>Besonderheiten:</strong> Allergien, Unverträglichkeiten oder persönliche Vorlieben</li>
            </ul>
            <p className="mt-4"><strong>Nützliche Funktionen:</strong></p>
            <ul className="mt-2 space-y-1 text-sm">
                <li>• <strong>Monatsübersicht:</strong> Kalendarische Darstellung der Teilnahme</li>
                <li>• <strong>Statistiken:</strong> Übersicht der häufigsten Menüwahlen und Teilnahmequote</li>
                <li>• <strong>An-/Abmeldung:</strong> Möglichkeit, den Schüler für bestimmte Tage an-/abzumelden</li>
                <li>• <strong>Essenspläne:</strong> Zukünftige Menüs einsehen und vorbestellen</li>
                <li>• <strong>Kostenübersicht:</strong> Kosten für vergangene Mahlzeiten und offene Beträge</li>
            </ul>
        </div>
    ),
    // Room Detail Page Help Content
    "room-detail": (
        <div>
            <p>In der <strong>Raumdetailansicht</strong> finden Sie:</p>
            <ul className="mt-3 space-y-2">
                <li>• <strong>Allgemeine Informationen:</strong> Name, Gebäude, Etage und Kapazität des Raums</li>
                <li>• <strong>Aktuelle Belegung:</strong> Ob und von wem der Raum aktuell genutzt wird</li>
                <li>• <strong>Belegungshistorie:</strong> Chronologische Übersicht aller vergangenen Nutzungen</li>
            </ul>
            <p className="mt-4"><strong>Funktionen und Informationen:</strong></p>
            <ul className="mt-2 space-y-1 text-sm">
                <li>• <strong>Statusanzeige:</strong> Farbliche Markierung ob der Raum frei oder belegt ist</li>
                <li>• <strong>Aktivitätsdetails:</strong> Welche Gruppe mit welcher Aktivität den Raum nutzt/genutzt hat</li>
                <li>• <strong>Zeitliche Informationen:</strong> Beginn, Ende und Dauer jeder Raumbelegung</li>
                <li>• <strong>Kategorie-Farbcodierung:</strong> Visuelle Unterscheidung der Raumnutzungsarten</li>
                <li>• <strong>Aufsichtspersonen:</strong> Verantwortliche Betreuer für jede Aktivität</li>
            </ul>
        </div>
    )
};

// Navigation Items als konstante Daten mit Hilfe-Inhalten
const NAV_ITEMS: NavItem[] = [
    {
        href: "/dashboard",
        label: "Home",
        icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z",
        helpContent: (
            <div>
                <p className="mb-4">Willkommen im <strong>Dashboard-Hilfesystem</strong>!</p>

                <h3 className="font-semibold text-lg mb-2">Verfügbare Funktionen:</h3>
                <ul className="list-disc list-inside space-y-1 mb-4">
                    <li><strong>Home</strong>: Übersicht über aktuelle Aktivitäten</li>
                    <li><strong>OGS Gruppe</strong>: Verwalten Sie Ganztagsgruppen</li>
                    <li><strong>Schüler</strong>: Suchen und verwalten Sie Schülerdaten</li>
                    <li><strong>Räume</strong>: Raumverwaltung und -zuweisung</li>
                    <li><strong>Aktivitäten</strong>: Erstellen und bearbeiten Sie Aktivitäten</li>
                    <li><strong>Statistiken</strong>: Einblick in wichtige Kennzahlen</li>
                    <li><strong>Vertretungen</strong>: Vertretungsplan verwalten</li>
                    <li><strong>Einstellungen</strong>: System konfigurieren</li>
                </ul>

                <p className="text-sm text-gray-600">
                    Klicken Sie auf einen <strong>Menüpunkt</strong>, um loszulegen.
                </p>
            </div>
        )
    },
    {
        href: "/ogs_groups",
        label: "OGS Gruppe",
        icon: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z",
        helpContent: (
            <div>
                <p>In der <strong>OGS-Gruppenansicht</strong> können Sie:</p>
                <ul className="mt-3 space-y-2">
                    <li>• <strong>Schüler verfolgen:</strong> Sehen, wo sich alle Kinder Ihrer Gruppe befinden</li>
                    <li>• <strong>Status einsehen:</strong> Im Gruppenraum, Schulhof, Toilette, Unterwegs oder Zuhause</li>
                    <li>• <strong>Filtern und suchen:</strong> Nach Name oder Jahrgang sortieren</li>
                </ul>
                <p className="mt-4"><strong>Statuserklärungen:</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                    <li>• <strong>Im Gruppenraum:</strong> Kind befindet sich im angegebenen OGS-Raum</li>
                    <li>• <strong>Schulhof:</strong> Kind ist auf dem Außengelände</li>
                    <li>• <strong>Toilette:</strong> Kind ist auf der Toilette</li>
                    <li>• <strong>Unterwegs:</strong> Kind ist in einer Aktivität oder wechselt gerade einen Raum</li>
                    <li>• <strong>Zuhause:</strong> Kind wurde abgeholt oder ist abgemeldet</li>
                </ul>
            </div>
        )
    },
    {
        href: "/students/search",
        label: "Schüler",
        icon: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z",
        helpContent: (
            <div>
                <p>In der <strong>Schülersuche</strong> können Sie:</p>
                <ul className="mt-3 space-y-2">
                    <li>• <strong>Schüler finden:</strong> Suche nach Namen, Klasse oder anderen Merkmalen</li>
                    <li>• <strong>Informationen einsehen:</strong> Persönliche Daten, Anwesenheiten und Aktivitäten</li>
                    <li>• <strong>Schnellfilter nutzen:</strong> Nach Jahrgangsstufe oder Anwesenheitsstatus filtern</li>
                </ul>
                <p className="mt-4"><strong>Tipps zur Suche:</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                    <li>• Nutzen Sie die <strong>Filteroptionen</strong> für spezifischere Ergebnisse</li>
                    <li>• Die Suche berücksichtigt Vor- und Nachnamen</li>
                    <li>• Klicken Sie auf einen Schüler, um sein Profil zu öffnen</li>
                    <li>• Exportieren Sie Ergebnisse mit dem Export-Button</li>
                </ul>
            </div>
        )
    },
    {
        href: "/rooms",
        label: "Räume",
        icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
        helpContent: (
            <div>
                <p>In der <strong>Raumverwaltung</strong> können Sie:</p>
                <ul className="mt-3 space-y-2">
                    <li>• <strong>Räume einsehen:</strong> Übersicht aller verfügbaren Räume</li>
                    <li>• <strong>Belegung prüfen:</strong> Aktuelle und geplante Raumbelegungen</li>
                    <li>• <strong>Räume zuweisen:</strong> Für Gruppen oder Aktivitäten</li>
                </ul>
                <p className="mt-4"><strong>Funktionen:</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                    <li>• <strong>Raumplan:</strong> Visuelle Darstellung der Raumbelegung</li>
                    <li>• <strong>Kalenderansicht:</strong> Zeitliche Übersicht der Raumbelegungen</li>
                    <li>• <strong>Konfliktprüfung:</strong> Automatische Erkennung von Doppelbelegungen</li>
                    <li>• <strong>Ausstattungsfilter:</strong> Suche nach Räumen mit bestimmter Ausstattung</li>
                </ul>
            </div>
        )
    },
    {
        href: "/activities",
        label: "Aktivitäten",
        icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
        helpContent: (
            <div>
                <p>In der <strong>Aktivitätenübersicht</strong> können Sie:</p>
                <ul className="mt-3 space-y-2">
                    <li>• <strong>Aktivitäten einsehen:</strong> Übersicht aller verfügbaren Aktivitäten</li>
                    <li>• <strong>Filtern:</strong> Nach Kategorie, Status, Teilnahmemöglichkeit und Gebäude</li>
                    <li>• <strong>Detailansicht:</strong> Informationen zu Leitung, Teilnehmerzahl und Ort</li>
                </ul>
                <p className="mt-4"><strong>Kategorien:</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                    <li>• <strong>Kreatives/Musik:</strong> Chor, Theaterprojekt und andere kreative Angebote</li>
                    <li>• <strong>NW/Technik:</strong> Informatik AG und technische Aktivitäten</li>
                    <li>• <strong>Bewegen/Ruhe:</strong> Sport-AGs wie Basketball</li>
                    <li>• <strong>Lernen:</strong> Lesegruppe und andere Bildungsangebote</li>
                    <li>• <strong>Hauswirtschaft:</strong> Praktische Aktivitäten</li>
                    <li>• <strong>Natur:</strong> Garten AG und naturnahe Angebote</li>
                    <li>• <strong>Spielen:</strong> Verschiedene Spiel- und Freizeitaktivitäten</li>
                </ul>
            </div>
        )
    },
    {
        href: "/statistics",
        label: "Statistiken",
        icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z",
        helpContent: (
            <div>
                <p>In der <strong>Statistikansicht</strong> finden Sie:</p>
                <ul className="mt-3 space-y-2">
                    <li>• <strong>Anwesenheitsstatistiken:</strong> Tägliche, wöchentliche und monatliche Übersichten</li>
                    <li>• <strong>Gruppenzahlen:</strong> Auslastung der OGS-Gruppen</li>
                    <li>• <strong>Aktivitätenanalyse:</strong> Beliebtheit und Teilnehmerzahlen</li>
                </ul>
                <p className="mt-4"><strong>Funktionen:</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                    <li>• <strong>Exportieren:</strong> Daten als Excel oder PDF herunterladen</li>
                    <li>• <strong>Zeitraumfilter:</strong> Daten für bestimmte Zeiträume anzeigen</li>
                    <li>• <strong>Vergleichsfunktion:</strong> Daten verschiedener Zeiträume vergleichen</li>
                    <li>• <strong>Diagramme:</strong> Visuelle Darstellung wichtiger Kennzahlen</li>
                </ul>
            </div>
        )
    },
    {
        href: "/substitutions",
        label: "Vertretungen",
        icon: "M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15",
        helpContent: (
            <div>
                <p>Im Bereich <strong>Vertretungen</strong> können Sie:</p>
                <ul className="mt-3 space-y-2">
                    <li>• <strong>Ausfälle erfassen:</strong> Abwesenheiten von Betreuern dokumentieren</li>
                    <li>• <strong>Vertretungen planen:</strong> Ersatzpersonal für abwesende Betreuer einteilen</li>
                    <li>• <strong>Konflikte erkennen:</strong> Unterbesetzte Gruppen identifizieren</li>
                </ul>
                <p className="mt-4"><strong>Weitere Funktionen:</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                    <li>• <strong>Kalenderansicht:</strong> Übersicht aller geplanten Vertretungen</li>
                    <li>• <strong>Benachrichtigungen:</strong> Betroffene Mitarbeiter informieren</li>
                    <li>• <strong>Auslastungsanzeige:</strong> Verfügbarkeit von Vertretungskräften prüfen</li>
                    <li>• <strong>Notfallplan:</strong> Vorgehen bei unerwarteten Ausfällen</li>
                </ul>
            </div>
        )
    },
    {
        href: "/database",
        label: "Datenbank",
        icon: "M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4",
        helpContent: (
            <div>
                <p>In der <strong>Datenbank</strong> können Sie:</p>
                <ul className="mt-3 space-y-2">
                    <li>• <strong>Gruppen verwalten:</strong> Erstellen und bearbeiten von OGS-Gruppen</li>
                    <li>• <strong>Aktivitäten organisieren:</strong> Aktivitäten erstellen und Schüler zuweisen</li>
                    <li>• <strong>Räume konfigurieren:</strong> Raumkapazitäten und Zuordnungen verwalten</li>
                    <li>• <strong>Schüler administrieren:</strong> Schülerdaten pflegen und aktualisieren</li>
                    <li>• <strong>Lehrer verwalten:</strong> Lehrer- und Betreuerdaten organisieren</li>
                </ul>
                <p className="mt-4"><strong>Erweiterte Funktionen:</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                    <li>• <strong>Datenimport:</strong> Massendaten importieren und synchronisieren</li>
                    <li>• <strong>Datenexport:</strong> Strukturierte Daten für Berichte exportieren</li>
                    <li>• <strong>Datenpflege:</strong> Systemweit Daten aktualisieren und bereinigen</li>
                    <li>• <strong>Beziehungen verwalten:</strong> Verknüpfungen zwischen Datensätzen pflegen</li>
                </ul>
            </div>
        )
    },
    {
        href: "/settings",
        label: "Einstellungen",
        icon: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z",
        helpContent: (
            <div>
                <p>In den <strong>Einstellungen</strong> können Sie:</p>
                <ul className="mt-3 space-y-2">
                    <li>• <strong>Benutzerprofil:</strong> Persönliche Daten und Zugangsdaten verwalten</li>
                    <li>• <strong>Benachrichtigungen:</strong> E-Mail und Push-Benachrichtigungen konfigurieren</li>
                    <li>• <strong>Erscheinungsbild:</strong> Anpassen der Benutzeroberfläche</li>
                </ul>
                <p className="mt-4"><strong>Administrative Einstellungen:</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                    <li>• <strong>Benutzer verwalten:</strong> Neue Konten erstellen und Berechtigungen zuweisen</li>
                    <li>• <strong>Schuljahr konfigurieren:</strong> Ferien und Feiertage festlegen</li>
                    <li>• <strong>Systemeinstellungen:</strong> Grundlegende Konfiguration des Systems</li>
                    <li>• <strong>Datensicherung:</strong> Backups erstellen und wiederherstellen</li>
                </ul>
            </div>
        )
    },
];

interface SidebarProps {
    className?: string;
}

function SidebarContent({ className = "" }: SidebarProps) {
    // Aktuelle Route ermitteln
    const pathname = usePathname();

    // Check if user has educational groups
    const { hasEducationalGroups, isLoading } = useHasEducationalGroups();

    // Filter navigation items based on user's educational groups
    const filteredNavItems = NAV_ITEMS.filter(item => {
        // Always show all items except "OGS Gruppe"
        if (item.href !== "/ogs_groups") {
            return true;
        }
        // Only show "OGS Gruppe" if user has educational groups
        // Don't show it while loading to avoid flickering
        return !isLoading && hasEducationalGroups;
    });

    // Funktion zur Überprüfung, ob ein Link aktiv ist
    const isActiveLink = (href: string) => {
        // Exakter Match für Dashboard
        if (href === "/dashboard") {
            return pathname === "/dashboard";
        }

        // Für andere Routen prüfen, ob der aktuelle Pfad mit dem Link-Pfad beginnt
        return pathname.startsWith(href);
    };

    // Den richtigen Hilfetext basierend auf der aktuellen Route finden
    const getActiveHelpContent = (): ReactNode => {
        // Spezielle Seiten prüfen
        if (/\/students\/[^\/]+$/.exec(pathname)) {
            return SPECIFIC_PAGE_HELP["student-detail"];
        } else if (/\/students\/[^\/]+\/room_history/.exec(pathname)) {
            return SPECIFIC_PAGE_HELP.room_history;
        } else if (/\/students\/[^\/]+\/feedback_history/.exec(pathname)) {
            return SPECIFIC_PAGE_HELP.feedback_history;
        } else if (/\/students\/[^\/]+\/mensa_history/.exec(pathname)) {
            return SPECIFIC_PAGE_HELP.mensa_history;
        } else if (/\/rooms\/[^\/]+$/.exec(pathname)) {
            return SPECIFIC_PAGE_HELP["room-detail"];
        }

        // Finde das NavItem, das der aktuellen Route entspricht
        const activeItem = filteredNavItems.find(item => {
            if (item.href === "/dashboard") {
                return pathname === "/dashboard";
            }
            return pathname.startsWith(item.href);
        });

        // Wenn ein aktives Item gefunden wurde, gib dessen Hilfetext zurück, sonst den Standard-Hilfetext
        if (activeItem?.helpContent) {
            return activeItem.helpContent;
        }
        return filteredNavItems[0]?.helpContent ?? (
            <div>
                <p><strong>Willkommen</strong></p>
                <p>Wählen Sie eine Option aus dem Menü, um weitere Informationen zu erhalten.</p>
            </div>
        );
    };

    // CSS-Klassen für aktive und normale Links
    const getLinkClasses = (href: string) => {
        const baseClasses = "flex items-center px-5 py-3 text-base font-medium rounded-lg transition-colors";
        const activeClasses = "bg-blue-50 text-blue-600 border-l-4 border-blue-600";
        const inactiveClasses = "text-gray-700 hover:bg-gray-100 hover:text-blue-600";

        return `${baseClasses} ${isActiveLink(href) ? activeClasses : inactiveClasses}`;
    };

    // Bestimmen des Hilfetitels basierend auf der aktuellen Route
    const getHelpButtonTitle = (): string => {
        if (/\/students\/[^\/]+$/.exec(pathname)) {
            return "Schülerprofil Hilfe";
        } else if (/\/students\/[^\/]+\/room_history/.exec(pathname)) {
            return "Raumverlauf Hilfe";
        } else if (/\/students\/[^\/]+\/feedback_history/.exec(pathname)) {
            return "Feedbackhistorie Hilfe";
        } else if (/\/students\/[^\/]+\/mensa_history/.exec(pathname)) {
            return "Mensaverlauf Hilfe";
        } else if (/\/rooms\/[^\/]+$/.exec(pathname)) {
            return "Raumdetail Hilfe";
        } else if (isActiveLink("/ogs_groups")) {
            return "OGS-Gruppenansicht Hilfe";
        } else if (isActiveLink("/rooms")) {
            return "Raumverwaltung Hilfe";
        } else if (isActiveLink("/database")) {
            return "Datenbank Hilfe";
        } else {
            return "Allgemeine Hilfe";
        }
    };

    return (
        <>
            <aside className={`w-64 bg-white border-r border-gray-200 min-h-screen overflow-y-auto ${className}`}>
                <div className="p-4">
                    <nav className="space-y-2">
                        {filteredNavItems.map((item) => (
                            <Link
                                key={item.href}
                                href={item.href}
                                className={getLinkClasses(item.href)}
                            >
                                <svg className="h-6 w-6 mr-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={item.icon} />
                                </svg>
                                {item.label}
                            </Link>
                        ))}
                    </nav>
                </div>
            </aside>

            {/* Hilfe-Button fixiert am unteren linken Bildschirmrand - dynamischer Hilfetext basierend auf der aktiven Route */}
            <div className="fixed bottom-0 left-0 z-50 w-64 p-4 bg-white border-t border-r border-gray-200">
                <div className="flex items-center justify-start">
                    <HelpButton
                        title={getHelpButtonTitle()}
                        content={getActiveHelpContent()}
                        buttonClassName="mr-2"
                    />
                    <span className="text-sm text-gray-600">Hilfe</span>
                </div>
            </div>
        </>
    );
}

export function Sidebar({ className = "" }: SidebarProps) {
    return (
        <UserContextProvider>
            <SidebarContent className={className} />
        </UserContextProvider>
    );
}