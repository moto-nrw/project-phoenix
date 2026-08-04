import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import {
  ConflictWarningsBanner,
  type ConflictBannerEntry,
} from "./conflict-warnings-banner";

const meta = {
  title: "timetable/ConflictWarningsBanner",
  component: ConflictWarningsBanner,
  args: {
    periodLabel: "diese Woche",
    onHide: () => undefined,
    onHideAll: () => undefined,
    onUnhide: () => undefined,
    onJump: () => undefined,
    revealHidden: false,
    onDismissHiddenPanel: () => undefined,
  },
} satisfies Meta<typeof ConflictWarningsBanner>;

export default meta;
type Story = StoryObj<typeof meta>;

const studentConflict: ConflictBannerEntry = {
  fingerprint: "aaaa1111bbbb2222cccc3333dddd4444",
  kind: "student",
  personName: "Emma Beispiel",
  dateISO: "2026-06-22",
  overlapStart: "14:30",
  overlapEnd: "15:00",
  instances: [
    { id: "1", title: "Fußball-AG" },
    { id: "2", title: "Lernzeit" },
  ],
};

const staffConflict: ConflictBannerEntry = {
  fingerprint: "eeee5555ffff6666aaaa7777bbbb8888",
  kind: "staff",
  personName: "Maria Muster",
  dateISO: "2026-06-23",
  overlapStart: "13:00",
  overlapEnd: "14:00",
  instances: [
    { id: "3", title: "Hausaufgaben" },
    { id: "4", title: "Bastel-AG" },
  ],
};

export const NoConflicts: Story = {
  args: {
    openConflicts: [],
    hiddenConflicts: [],
  },
};

export const SingleConflict: Story = {
  args: {
    openConflicts: [studentConflict],
    hiddenConflicts: [],
  },
};

export const MultipleConflicts: Story = {
  args: {
    openConflicts: [studentConflict, staffConflict],
    hiddenConflicts: [],
  },
};

export const WithHiddenConflicts: Story = {
  args: {
    openConflicts: [studentConflict],
    hiddenConflicts: [staffConflict],
  },
};

/**
 * All conflicts acknowledged: the banner itself renders NOTHING (#2139 —
 * dismissed warnings must not reappear). This story shows the hidden-conflicts
 * panel as the page's ⋮ menu opens it (`revealHidden`).
 */
export const AllHiddenPanelRevealed: Story = {
  args: {
    openConflicts: [],
    hiddenConflicts: [studentConflict, staffConflict],
    revealHidden: true,
  },
};
