import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { AttachmentList } from "./attachment-list";

const ATTACHMENTS = [
  {
    id: "1",
    filename: "Elternbrief-Herbstferien.pdf",
    size_bytes: 248_312,
    content_type: "application/pdf",
  },
  {
    id: "2",
    filename: "Anmeldung-Ferienbetreuung.docx",
    size_bytes: 1_843_200,
    content_type:
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  },
  {
    id: "3",
    filename: "Foto-Ausflug.jpg",
    size_bytes: 512,
    content_type: "image/jpeg",
  },
];

const meta = {
  title: "components/ui/AttachmentList",
  component: AttachmentList,
  args: {
    attachments: ATTACHMENTS,
    downloadUrl: (id: string) => `/api/attachments/${id}`,
  },
  decorators: [
    (StoryFn) => (
      <div className="max-w-xl">
        <StoryFn />
      </div>
    ),
  ],
} satisfies Meta<typeof AttachmentList>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Removable: Story = {
  args: {
    onRemove: () => {
      // no-op for story
    },
  },
};

export const FileActions: Story = {
  args: { fileActions: true },
};
