import { StatusBadge } from "~/components/ui/status-badge";

/**
 * Marks a test Träger (#4658) on every page of the Träger portal, so nobody
 * mistakes the invented sample data for real children or enters real data.
 *
 * A strip above the shared header (#5145), like the demo strip of the staff
 * portal; it scrolls away with the page and the header stays sticky. The
 * shell shows it only for a test Träger.
 */
export function TraegerTestSystemNotice() {
  return (
    <div
      data-traeger-test-system
      className="relative z-10 flex items-center gap-3 border-b border-gray-200 bg-white px-4 py-2 sm:px-6 lg:px-8"
    >
      <span className="shrink-0">
        <StatusBadge label="Testsystem" tone="orange" />
      </span>
      <p className="min-w-0 text-sm text-gray-700">
        Hier probieren Sie moto aus. Alle Kinder sind erfunden. Tragen Sie hier
        keine echten Daten ein.
      </p>
    </div>
  );
}
