import { cn } from "~/lib/utils";
import { listRowClasses } from "./list-row-styles";

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
        className,
        SKELETON_SHAPES[shape],
      )}
      {...props}
    />
  );
}

/** Platzhalter für eine `ListRow`: Bezeichnung links, Inhalt rechts. */
function ListRowSkeleton({
  labelClassName = "w-24",
  valueClassName = "flex-1",
}: Readonly<{
  labelClassName?: string;
  valueClassName?: string;
}>) {
  return (
    <div className={listRowClasses()}>
      <Skeleton className={cn("h-4", labelClassName)} />
      <Skeleton className={cn("h-4", valueClassName)} />
    </div>
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
        <ListRowSkeleton
          key={index}
          labelClassName={labelClassName}
          valueClassName={valueClassName}
        />
      ))}
    </div>
  );
}

export { ListRowSkeleton, Skeleton, SkeletonRows };
