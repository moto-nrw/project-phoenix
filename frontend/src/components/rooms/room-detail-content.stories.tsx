import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { RoomDetailContent, RoomDetailLoader } from "./room-detail-content";

// RoomDetailContent renders <StudentsInRoomSection>, which needs
// ToastProvider (see students-in-room-section.stories.tsx) and fetches its
// own data via SWR. There is no backend in Storybook, so that section
// renders its own error state — expected and harmless for demoing the
// layout/chrome of this component.
const withToast = (Story: () => React.JSX.Element) => (
  <ToastProvider>
    <Story />
  </ToastProvider>
);

const freeRoom = {
  id: "1",
  name: "Gruppenraum Sonne",
  building: "Hauptgebäude",
  floor: 1,
  capacity: 20,
  category: "Gruppenraum",
  isOccupied: false,
};

const occupiedRoom = {
  id: "2",
  name: "Sporthalle",
  building: "Nebengebäude",
  floor: 0,
  capacity: 40,
  category: "Sporthalle",
  isOccupied: true,
  groupName: "Fußball AG",
  supervisorName: "Frau Meier",
  studentCount: 12,
};

const history = [
  {
    sessionId: "1",
    startedAt: "2026-06-30T09:00:00Z",
    endedAt: "2026-06-30T10:30:00Z",
    durationMinutes: 90,
    activityName: "Fußball AG",
    supervisorName: "Frau Meier",
    studentCount: 12,
  },
  {
    sessionId: "2",
    startedAt: "2026-06-30T13:00:00Z",
    endedAt: null,
    durationMinutes: null,
    activityName: "Freispiel",
    supervisorName: "Herr Schmidt",
    studentCount: 8,
  },
];

const meta = {
  title: "rooms/RoomDetailContent",
  component: RoomDetailContent,
  decorators: [withToast],
  args: {
    room: freeRoom,
    history: [],
  },
} satisfies Meta<typeof RoomDetailContent>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Free room, no occupancy history — the subpage layout (no headerAction). */
export const Default: Story = {};

/** Occupied room with active activity, supervisor, and student count. */
export const Occupied: Story = {
  args: {
    room: occupiedRoom,
    history,
  },
};

/** Modal context: a headerAction is passed (renders the sticky header + close button slot). */
export const ModalContext: Story = {
  args: {
    room: occupiedRoom,
    history,
    headerAction: (
      <button type="button" className="rounded-md p-2 text-gray-500">
        Schließen
      </button>
    ),
  },
};

/**
 * historyDisabled=true hides the "Belegungshistorie" section even though
 * history entries are present (issue #1425 — tenant has
 * gdpr.attendance_log_enabled = false).
 */
export const HistoryDisabled: Story = {
  args: {
    room: occupiedRoom,
    history,
    historyDisabled: true,
  },
};

/**
 * RoomDetailLoader fetches room + history via SWR on mount. There is no
 * backend in Storybook, so this demonstrates the loading skeleton followed
 * by the error state ("Raum nicht gefunden" / fetch error) rather than a
 * populated room — the same behavior any unmocked network dependency shows
 * here (see students-in-room-section.stories.tsx for the same pattern).
 */
export const LoaderErrorState: StoryObj<typeof RoomDetailLoader> = {
  render: (args) => <RoomDetailLoader {...args} />,
  args: {
    roomId: "1",
  },
  decorators: [withToast],
};
