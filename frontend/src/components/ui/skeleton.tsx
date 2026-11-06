import { cn } from "~/lib/utils";

/**
 * Radius der Platzhalter nach dem Radius-Maßstab des Kits. `line` ist die
 * Textzeile, `round` Avatar, Punkt oder Pille, `control` ein Bedienelement,
 * `inner` eine innere Fläche, `card` eine Karte, `none` die Zelle eines Rasters
 * ohne eigene Ecken. Der Radius kommt nie über `className`
 * (bauart/no-own-skeleton, #5588).
 */
const SKELETON_SHAPES = {
  line: "rounded-md",
  round: "rounded-full",
  control: "rounded-lg",
  inner: "rounded-xl",
  card: "rounded-2xl",
  none: "rounded-none",
} as const;

function Skeleton({
  shape = "line",
  className,
  ...props
}: Readonly<
  React.HTMLAttributes<HTMLDivElement> & {
    shape?: keyof typeof SKELETON_SHAPES;
  }
>) {
  return (
    <div
      className={cn(
        "animate-pulse bg-gray-200 motion-reduce:animate-none",
        SKELETON_SHAPES[shape],
        className,
      )}
      {...props}
    />
  );
}

/**
 * Zeilen einer kurzen Liste in einer Karte: Bezeichnung links, Inhalt rechts.
 * Für Blöcke fester Höhe: Mit Avatar und zwei Textzeilen ragt `ListSkeleton` über
 * ihren Rand, beim Laden blitzt dann ein Scrollbalken auf. Rein darstellend; `SkeletonRegion` oder die Karte
 * sagt das Laden an.
 */
function SkeletonRows({
  rows = 3,
  labelClassName = "w-24",
  valueClassName = "flex-1",
}: Readonly<{
  rows?: number;
  labelClassName?: string;
  valueClassName?: string;
}>) {
  return (
    <div className="space-y-2" aria-hidden="true">
      {Array.from({ length: rows }, (_, index) => (
        <div
          key={index}
          className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2"
        >
          <Skeleton className={cn("h-4", labelClassName)} />
          <Skeleton className={cn("h-4", valueClassName)} />
        </div>
      ))}
    </div>
  );
}

export { Skeleton, SkeletonRows };
