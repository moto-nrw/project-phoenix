// PersonalInfoEditForm - extracted from student detail page
"use client";

interface ExtendedStudent {
  first_name: string;
  second_name: string;
  school_class: string;
  birthday?: string;
  buskind?: boolean;
  pickup_status?: string;
  health_info?: string;
  supervisor_notes?: string;
  extra_info?: string;
  sick?: boolean;
}

interface PersonalInfoEditFormProps {
  readonly editedStudent: ExtendedStudent;
  readonly onStudentChange: (student: ExtendedStudent) => void;
  readonly onSave: () => void;
  readonly onCancel: () => void;
}

export function PersonalInfoEditForm({
  editedStudent,
  onStudentChange,
  onSave,
  onCancel,
}: PersonalInfoEditFormProps) {
  const updateField = <K extends keyof ExtendedStudent>(
    field: K,
    value: ExtendedStudent[K],
  ) => {
    onStudentChange({ ...editedStudent, [field]: value });
  };

  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 sm:h-10 sm:w-10">
          <PersonIcon />
        </div>
        <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
          Persönliche Informationen
        </h2>
      </div>

      <div className="space-y-3">
        <TextInput
          label="Vorname"
          value={editedStudent.first_name}
          onChange={(value) => updateField("first_name", value)}
        />
        <TextInput
          label="Nachname"
          value={editedStudent.second_name}
          onChange={(value) => updateField("second_name", value)}
        />
        <TextInput
          label="Klasse"
          value={editedStudent.school_class}
          onChange={(value) => updateField("school_class", value)}
        />
        <DateInput
          label="Geburtsdatum"
          value={editedStudent.birthday}
          onChange={(value) => updateField("birthday", value)}
        />
        <SelectInput
          label="Buskind"
          value={editedStudent.buskind ? "true" : "false"}
          onChange={(value) => updateField("buskind", value === "true")}
          options={[
            { value: "false", label: "Nein" },
            { value: "true", label: "Ja" },
          ]}
        />
        <SelectInput
          label="Abholstatus"
          value={editedStudent.pickup_status ?? ""}
          onChange={(value) => updateField("pickup_status", value || undefined)}
          options={[
            { value: "", label: "Nicht gesetzt" },
            {
              value: "Geht alleine nach Hause",
              label: "Geht alleine nach Hause",
            },
            { value: "Wird abgeholt", label: "Wird abgeholt" },
          ]}
        />
        <SickToggle
          isSick={editedStudent.sick ?? false}
          onChange={(value) => updateField("sick", value)}
        />
        <TextareaInput
          label="Gesundheitsinformationen"
          value={editedStudent.health_info ?? ""}
          onChange={(value) => updateField("health_info", value)}
          placeholder="Allergien, Medikamente, wichtige medizinische Informationen"
          rows={3}
        />
        <TextareaInput
          label="Betreuernotizen"
          value={editedStudent.supervisor_notes ?? ""}
          onChange={(value) => updateField("supervisor_notes", value)}
          placeholder="Notizen für Betreuer"
          rows={3}
        />
        <TextareaInput
          label="Elternnotizen"
          value={editedStudent.extra_info ?? ""}
          onChange={(value) => updateField("extra_info", value)}
          placeholder="Notizen der Eltern"
          rows={2}
        />

        {/* Action Buttons */}
        <div className="flex flex-col-reverse gap-2 pt-4 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onCancel}
            className="inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:bg-gray-50 hover:shadow-lg active:scale-[0.99] sm:hover:scale-[1.01]"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={onSave}
            className="inline-flex items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-[0.99] sm:hover:scale-[1.01]"
          >
            Speichern
          </button>
        </div>
      </div>
    </div>
  );
}

// Form Input Components
interface TextInputProps {
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
}

function TextInput({ label, value, onChange }: TextInputProps) {
  return (
    <div>
      <label className="mb-1 block text-xs text-gray-500">{label}</label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
      />
    </div>
  );
}

interface DateInputProps {
  readonly label: string;
  readonly value?: string;
  readonly onChange: (value: string) => void;
}

function DateInput({ label, value, onChange }: DateInputProps) {
  return (
    <div>
      <label className="mb-1 block text-xs text-gray-500">{label}</label>
      <input
        type="date"
        value={value ? value.split("T")[0] : ""}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
      />
    </div>
  );
}

interface SelectInputProps {
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly options: { value: string; label: string }[];
}

function SelectInput({ label, value, onChange, options }: SelectInputProps) {
  return (
    <div>
      <label className="mb-1 block text-xs text-gray-500">{label}</label>
      <div className="relative">
        <select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-full appearance-none rounded-lg border border-gray-300 bg-white px-3 py-2.5 pr-10 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
        >
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <ChevronDownIcon />
      </div>
    </div>
  );
}

interface TextareaInputProps {
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly placeholder?: string;
  readonly rows?: number;
}

function TextareaInput({
  label,
  value,
  onChange,
  placeholder,
  rows = 3,
}: TextareaInputProps) {
  return (
    <div>
      <label className="mb-1 block text-xs text-gray-500">{label}</label>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="min-h-[60px] w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
        rows={rows}
        placeholder={placeholder}
      />
    </div>
  );
}

interface SickToggleProps {
  readonly isSick: boolean;
  readonly onChange: (value: boolean) => void;
}

function SickToggle({ isSick, onChange }: SickToggleProps) {
  return (
    <div
      className={`rounded-lg border p-4 transition-colors ${
        isSick ? "border-amber-200 bg-amber-50" : "border-gray-200 bg-gray-50"
      }`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div
            className={`flex h-9 w-9 items-center justify-center rounded-lg ${
              isSick
                ? "bg-amber-100 text-amber-600"
                : "bg-gray-200 text-gray-500"
            }`}
          >
            <WarningIcon />
          </div>
          <div>
            <p
              className={`text-sm font-medium ${
                isSick ? "text-amber-900" : "text-gray-700"
              }`}
            >
              Kind krankmelden
            </p>
            <p className="text-xs text-gray-500">
              Wird beim nächsten Check-in zurückgesetzt
            </p>
          </div>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={isSick}
          onClick={() => onChange(!isSick)}
          className={`relative inline-flex h-7 w-12 flex-shrink-0 items-center rounded-full transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-500 focus-visible:ring-offset-2 ${
            isSick ? "bg-amber-500" : "bg-gray-300"
          }`}
        >
          <span
            className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition-transform duration-200 ${
              isSick ? "translate-x-6" : "translate-x-1"
            }`}
          />
        </button>
      </div>
    </div>
  );
}

// Icons
function PersonIcon() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
      />
    </svg>
  );
}

function ChevronDownIcon() {
  return (
    <svg
      className="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 9l-7 7-7-7"
      />
    </svg>
  );
}

function WarningIcon() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
      />
    </svg>
  );
}
