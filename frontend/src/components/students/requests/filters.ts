import type {
  AggregatedRequestStatus,
  AggregatedRequestType,
} from "~/lib/change-request-list-api";

/**
 * Was die Anfragenliste zeigen soll. Suche, Arten und Zeitraum wirken
 * serverseitig; die drei `include*`-Schalter entscheiden, welche Quellen
 * überhaupt abgefragt werden dürfen (#2432).
 */
export interface AggregatedRequestFilters {
  readonly search: string;
  /**
   * Nur die Einträge dieses Kindes — das Änderungsprotokoll der Kinderkartei
   * (#2437). Ohne Angabe: alle Kinder, die die Person sehen darf.
   */
  readonly studentId?: string;
  /**
   * Darf der Aggregator über die vier Kinderdaten-Arten abgefragt werden? Er
   * verlangt users:update oder users:absence; wer nur Anmeldungsänderungen
   * entscheidet, bekäme sonst für die ganze Liste einen 403. Ohne Angabe ja —
   * er ist die Hauptquelle der Liste.
   */
  readonly includeAggregated?: boolean;
  /**
   * Dürfen Anmeldungsänderungen mitgeladen werden? Sie hängen an config:manage
   * und kommen aus einem eigenen Endpunkt; ohne das Recht bleibt die Quelle
   * weg, statt der Seite einen 403 einzuhandeln.
   */
  readonly includeEnrollment?: boolean;
  /** Offene Komplett-Abmeldungen; verlangt users:delete. */
  readonly includeCareWithdrawals?: boolean;
  readonly canManageFamilyProtection?: boolean;
  /** Leer = alle Arten. */
  readonly types: readonly AggregatedRequestType[];
  /** Nur Historie; leer = alle Status. */
  readonly statuses: readonly AggregatedRequestStatus[];
  /** Nur Historie, YYYY-MM-DD. */
  readonly from?: string;
  /** Nur Historie, YYYY-MM-DD. */
  readonly to?: string;
}
