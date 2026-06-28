import type { RequestDiffEntry } from "~/lib/messaging-status";

/**
 * The "current → requested" diff panel for a parent request, plus the optional
 * decision-reason line. Rendered identically on the staff review card and the
 * parent's own request card — only the heading differs ("Änderungen" for staff,
 * "Ihre Änderungswünsche" for the parent), and the action buttons / i18n live in
 * the surrounding cards. Extracted so the two portals share one source of truth
 * (alongside the shared ChatBubble), instead of the markup being copy-pasted.
 */
export function RequestDiffPanel({
  diff,
  heading,
  decisionReason,
}: Readonly<{
  diff?: readonly RequestDiffEntry[];
  heading: string;
  decisionReason?: string;
}>) {
  return (
    <>
      {diff && diff.length > 0 ? (
        <div className="mt-3 space-y-2 rounded-lg bg-gray-50 p-3">
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {heading}
          </p>
          {diff.map((entry) => (
            <div key={entry.label} className="text-sm">
              <span className="text-xs text-gray-500">{entry.label}</span>
              <div className="flex flex-wrap items-baseline gap-2">
                <span className="text-gray-400 line-through">{entry.old}</span>
                <span className="text-gray-400" aria-hidden="true">
                  →
                </span>
                <span className="font-medium text-gray-900">{entry.new}</span>
              </div>
            </div>
          ))}
        </div>
      ) : null}
      {decisionReason ? (
        <p className="mt-3 rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-700">
          Grund: {decisionReason}
        </p>
      ) : null}
    </>
  );
}
