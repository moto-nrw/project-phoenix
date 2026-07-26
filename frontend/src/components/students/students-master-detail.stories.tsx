import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { StudentsMasterDetail } from "./students-master-detail";
import type { Student } from "~/lib/api";

const students: Student[] = [
  {
    id: "1",
    name: "Anna Beispiel",
    first_name: "Anna",
    second_name: "Beispiel",
    school_class: "3a",
    group_name: "Gruppe Sonne",
    current_location: "anwesend",
  },
  {
    id: "2",
    name: "Ben Muster",
    first_name: "Ben",
    second_name: "Muster",
    school_class: "3a",
    group_name: "Gruppe Sonne",
    current_location: "abwesend",
  },
  {
    id: "3",
    name: "Clara Test",
    first_name: "Clara",
    second_name: "Test",
    school_class: "4b",
    group_name: "Gruppe Mond",
    current_location: "anwesend",
  },
];

const groups = [
  { value: "sonne", label: "Gruppe Sonne" },
  { value: "mond", label: "Gruppe Mond" },
];

function StudentsMasterDetailStory() {
  const [selectedId, setSelectedId] = useState<string | null>("1");
  return (
    <div style={{ height: "600px" }}>
      <StudentsMasterDetail
        students={students}
        selectedId={selectedId}
        onSelect={setSelectedId}
        grouping="class"
        studentsWithArrival={new Set(["1", "3"])}
        arrivalSummaryById={new Map([["1", "08:00 - 16:00"]])}
        onArrivalDataChanged={() => {
          // no-op for the story
        }}
        groups={groups}
        onUpdateStudent={() => Promise.resolve()}
        canViewEnrollments={false}
      />
    </div>
  );
}

const meta = {
  title: "students/StudentsMasterDetail",
  component: StudentsMasterDetailStory,
} satisfies Meta<typeof StudentsMasterDetailStory>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const NoneSelected: Story = {
  render: () => {
    function NoneSelectedStory() {
      const [selectedId, setSelectedId] = useState<string | null>(null);
      return (
        <div style={{ height: "600px" }}>
          <StudentsMasterDetail
            students={students}
            selectedId={selectedId}
            onSelect={setSelectedId}
            grouping="none"
            studentsWithArrival={new Set(["1", "3"])}
            arrivalSummaryById={new Map()}
            onArrivalDataChanged={() => {
              // no-op for the story
            }}
            groups={groups}
            onUpdateStudent={() => Promise.resolve()}
          />
        </div>
      );
    }
    return <NoneSelectedStory />;
  },
};

export const EmptyList: Story = {
  render: () => {
    function EmptyListStory() {
      const [selectedId, setSelectedId] = useState<string | null>(null);
      return (
        <div style={{ height: "600px" }}>
          <StudentsMasterDetail
            students={[]}
            selectedId={selectedId}
            onSelect={setSelectedId}
            grouping="none"
            studentsWithArrival={new Set()}
            arrivalSummaryById={new Map()}
            onArrivalDataChanged={() => {
              // no-op for the story
            }}
            groups={[]}
            onUpdateStudent={() => Promise.resolve()}
          />
        </div>
      );
    }
    return <EmptyListStory />;
  },
};
