import {
  appChapters,
  nfcQuickstartChapters,
  nfcChapters,
  setupChapters,
  type GuideChapter,
} from "./guide-data";

/**
 * One searchable entry in the help search index. Records are derived
 * automatically from the guide data (see {@link buildHelpSearchIndex}), so every
 * new chapter or step becomes searchable without touching this file.
 */
export interface HelpSearchRecord {
  /** Stable unique key: `${guidePath}#${anchor}`. */
  readonly id: string;
  /** Deep link to the chapter or step, e.g. `/help/setup#konto-erstellen`. */
  readonly href: string;
  /** Which guide this belongs to, e.g. "Ersteinrichtung". */
  readonly guideLabel: string;
  /** Parent chapter title (equals `title` for chapter-level records). */
  readonly chapterTitle: string;
  /** Primary heading shown in results (step title or chapter title). */
  readonly title: string;
  /** Short supporting line (step summary or chapter description). */
  readonly summary: string;
  /** Concatenated detail text for fuzzy matching (steps, checklist, callout). */
  readonly body: string;
}

/** A guide page and the chapters it renders, with display metadata. */
interface GuideSource {
  /** Route path of the guide page. */
  readonly path: string;
  /** Human label for the guide, shown as the result breadcrumb. */
  readonly label: string;
  readonly chapters: readonly GuideChapter[];
}

/**
 * The three guide pages. Paths and labels mirror `guideEntryPoints` /
 * `pdfByPath`. The `help-search-sync.test.ts` guard fails if a path here drifts
 * from `guideEntryPoints`, so a renamed or added guide can never silently drop
 * out of search.
 */
export const GUIDE_SOURCES: readonly GuideSource[] = [
  { path: "/help/setup", label: "Ersteinrichtung", chapters: setupChapters },
  { path: "/help/features", label: "Die App im Alltag", chapters: appChapters },
  {
    path: "/help/nfc/erste-schritte",
    label: "NFC Erste Schritte",
    chapters: nfcQuickstartChapters,
  },
  { path: "/help/nfc", label: "NFC & Tablets", chapters: nfcChapters },
];

/** Paths that MUST exist as guide entry points (consumed by the sync test). */
export const helpGuidePaths: readonly string[] = GUIDE_SOURCES.map(
  (g) => g.path,
);

/** Unicode combining diacritical marks (U+0300–U+036F), stripped after NFD. */
const COMBINING_MARKS = new RegExp("[\\u0300-\\u036f]", "g");

/**
 * Length-preserving fold: NFD-decompose, drop combining marks, lower-case.
 * German precomposed umlauts collapse 1:1 (ä→a, ö→o, ü→u), so folded indices
 * map straight back onto the original text — this is what highlight ranges rely
 * on. Do NOT add length-changing rewrites (ß→ss) here; use {@link foldForMatch}.
 *
 * The 1:1 property only holds when the input is NFC (precomposed). The index
 * normalises display text to NFC at build time (see {@link buildHelpSearchIndex})
 * so this invariant is unconditional regardless of how the guide source is encoded.
 */
export function fold(value: string): string {
  return value.normalize("NFD").replace(COMBINING_MARKS, "").toLowerCase();
}

/**
 * Aggressive fold for FUZZY MATCHING only (never for highlighting). On top of
 * {@link fold} it expands the sharp-S so "strasse" finds "Straße" and vice
 * versa. This changes string length, so it must not be used to compute
 * highlight offsets — only to feed the Fuse index and the query tokens.
 */
export function foldForMatch(value: string): string {
  return fold(value).replace(/ß/g, "ss");
}

/** Collect every piece of step text worth matching into one haystack string. */
function stepBody(step: GuideChapter["steps"][number]): string {
  const parts: string[] = [];
  if (step.steps) parts.push(...step.steps);
  if (step.checklist) parts.push(...step.checklist);
  if (step.callout) parts.push(step.callout.title, step.callout.body);
  if (step.screenshot) parts.push(step.screenshot);
  if (step.gallery) parts.push(...step.gallery.map((g) => g.caption));
  return parts.join(" ");
}

/**
 * Flatten all guides into a flat, JSON-serialisable search index. Pure and
 * side-effect free so it can be unit tested and built once at module load.
 *
 * One record per step card ("Kachel") — chapters are not searchable on their
 * own. The parent chapter's heading is exposed as its own field (`chapterTitle`
 * → `chapterFold` key), and its intro is folded into the card's body haystack,
 * so a section-level query still surfaces the cards under it.
 */
export function buildHelpSearchIndex(): HelpSearchRecord[] {
  const records: HelpSearchRecord[] = [];

  // Normalise display + match text to NFC so {@link fold} stays length-preserving
  // (decomposed `a`+U+0308 would fold 2→1 and shift every highlight offset).
  const nfc = (value: string) => value.normalize("NFC");

  for (const guide of GUIDE_SOURCES) {
    for (const chapter of guide.chapters) {
      for (const step of chapter.steps) {
        records.push({
          id: `${guide.path}#${step.id}`,
          href: `${guide.path}#${step.id}`,
          guideLabel: guide.label,
          chapterTitle: nfc(chapter.title),
          title: nfc(step.title),
          summary: nfc(step.summary),
          body: nfc([chapter.description, stepBody(step)].join(" ")),
        });
      }
    }
  }

  return records;
}

/** Prebuilt index. Stable across the app lifetime; the guide data is static. */
export const helpSearchIndex: readonly HelpSearchRecord[] =
  buildHelpSearchIndex();

/**
 * A record paired with its match-folded fields. We fold ONCE here at module
 * load instead of inside Fuse's `getFn` (which re-folds on every keystroke and
 * every record). At a few dozen cards the difference is noise; once the help
 * grows to hundreds, precomputing keeps typing latency flat. `record` carries
 * the original, unfolded text for display + highlighting.
 */
export interface IndexedHelpRecord {
  readonly record: HelpSearchRecord;
  readonly titleFold: string;
  readonly chapterFold: string;
  readonly summaryFold: string;
  readonly bodyFold: string;
}

/** Fuse searches over these precomputed folded fields. */
export const indexedHelpSearchIndex: readonly IndexedHelpRecord[] =
  helpSearchIndex.map((record) => ({
    record,
    titleFold: foldForMatch(record.title),
    chapterFold: foldForMatch(record.chapterTitle),
    summaryFold: foldForMatch(record.summary),
    bodyFold: foldForMatch(record.body),
  }));
