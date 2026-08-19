import { cn } from "~/lib/utils";

function Skeleton({
  className,
  ...props
}: Readonly<React.HTMLAttributes<HTMLDivElement>>) {
  return (
    <div
      className={cn(
        "animate-pulse rounded-md bg-gray-200 motion-reduce:animate-none",
        className,
      )}
      {...props}
    />
  );
}

export { Skeleton };
