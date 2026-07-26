import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { StudentsInRoomSection } from "~/components/rooms/students-in-room-section";

const meta = {
  title: "rooms/StudentsInRoomSection",
  component: StudentsInRoomSection,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  parameters: {
    // The component fetches /api/students, /api/active/groups and
    // /api/rooms via SWR on mount. In Storybook there is no backend, so
    // these requests fail and the section renders its error state
    // ("Die Liste der Kinder konnte nicht geladen werden.") — expected
    // and harmless for demoing the layout/chrome of the section.
    docs: { description: { component: "" } },
  },
  args: {
    roomId: "1",
    roomName: "Gruppenraum Sonne",
  },
} satisfies Meta<typeof StudentsInRoomSection>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default state as embedded in the room-detail slide-over. No backend is
 * available in Storybook, so this renders the section's error state.
 */
export const Default: Story = {};
