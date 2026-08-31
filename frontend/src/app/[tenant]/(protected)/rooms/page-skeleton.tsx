"use client";

// Brand color hex codes via LOCATION_COLORS (CLAUDE.md §0,
// lib/location-helper.ts): OTHER_ROOM (#5080D8) for blue accents,
// DANGER (#DC2626) for occupied/error, GROUP_ROOM (#83CD2D) for free
// (with #4a7a15 text for AA contrast on the tinted background).

// Single skeleton card that matches the populated room card's outer
// shell: same rounded-2xl, same min-h-[180px], same flex layout so the
// page doesn't reshuffle on swap. Pulse blocks stand in for title row,
// meta line, status pill, two middle rows, and the footer hint.
function RoomCardSkeleton() {
  return (
    <div className="moto-content-surface relative overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <div className="bg-moto-blue absolute inset-0 rounded-2xl opacity-[0.03]"></div>
      <div className="relative flex min-h-[180px] flex-col p-6">
        <div className="mb-3 flex items-start justify-between">
          <div className="min-w-0 flex-1 space-y-2">
            <div className="h-5 w-2/3 animate-pulse rounded bg-gray-200" />
            <div className="h-3 w-1/3 animate-pulse rounded bg-gray-200" />
          </div>
          <div className="ml-3 h-6 w-16 flex-shrink-0 animate-pulse rounded-full bg-gray-200" />
        </div>
        <div className="flex-1 space-y-2">
          <div className="h-3 w-3/4 animate-pulse rounded bg-gray-200" />
          <div className="h-3 w-1/2 animate-pulse rounded bg-gray-200" />
        </div>
        <div className="mt-2 h-3 w-24 animate-pulse rounded bg-gray-200" />
      </div>
    </div>
  );
}

export function RoomsGridSkeleton() {
  // Eight cards covers two rows on the largest grid (2xl: 4 columns);
  // smaller breakpoints fill more rows naturally. Same gap + column
  // breakpoints as the populated grid below so the swap is purely a
  // child-level change, not a container reshape.
  return (
    <output
      aria-label="Räume werden geladen"
      data-testid="rooms-grid-skeleton"
      className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
    >
      {Array.from({ length: 8 }).map((_, i) => (
        <RoomCardSkeleton key={i} />
      ))}
    </output>
  );
}
