import { Download } from "lucide-react";

// Links to the pre-generated, clean PDF for this guide (built in CI from the
// HTML page via pnpm run generate:guides). Replaces the old window.print()
// flow, which could not suppress the browser's own date/URL/title header and
// footer.
export function GuidePdfButton({
  href,
  download,
}: {
  readonly href: string;
  /** German filename the visitor sees when saving , independent of the
      ASCII served path. */
  readonly download: string;
}) {
  return (
    <a
      href={href}
      download={download}
      className="inline-flex h-10 items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold whitespace-nowrap text-gray-700 shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      aria-label="PDF herunterladen"
    >
      <Download className="h-4 w-4" aria-hidden="true" />
      <span className="hidden sm:inline">PDF herunterladen</span>
    </a>
  );
}
