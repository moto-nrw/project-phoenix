import { FileUploadInput } from "~/components/enrollment/enrollment-file-upload";
import { CustomDateFieldInput } from "~/components/enrollment/custom-date-field-input";
import { CustomFieldInput as SharedCustomFieldInput } from "~/components/enrollment/custom-field-input";
export { InfoBlock } from "~/components/enrollment/custom-field-input";
import {
  ContactListInput,
  PhoneListInput,
} from "./enrollment-form-contact-inputs";
import type { CustomFieldInputProps } from "./enrollment-form-shared";
import {
  WeekdayBooleanInput,
  WeekdayModeInput,
  WeekdayMultiModeInput,
  WeekdayScheduleInput,
} from "./enrollment-form-weekday-inputs";

export function CustomFieldInput({
  field,
  inputName,
  companionNoteName,
  value,
  onChange,
  error,
  tr,
  relevantDays,
  careConstrained,
  companionNote,
  onCompanionNoteChange,
  companionNoteError,
  singleMode,
}: CustomFieldInputProps) {
  const renderStructured = () => {
    if (field.type === "phone_list") {
      return (
        <PhoneListInput
          field={field}
          inputName={inputName}
          value={value}
          onChange={onChange}
          error={error}
          tr={tr}
        />
      );
    }
    if (field.type === "weekday_schedule") {
      return (
        <WeekdayScheduleInput
          field={field}
          inputName={inputName}
          value={value}
          onChange={onChange}
          error={error}
          tr={tr}
          relevantDays={relevantDays}
          careConstrained={careConstrained}
        />
      );
    }
    if (field.type === "weekday_boolean") {
      return (
        <WeekdayBooleanInput
          field={field}
          inputName={inputName}
          value={value}
          onChange={onChange}
          error={error}
          tr={tr}
        />
      );
    }
    if (field.type === "weekday_mode") {
      return (
        <WeekdayModeInput
          field={field}
          inputName={inputName}
          value={value}
          onChange={onChange}
          error={error}
          tr={tr}
        />
      );
    }
    if (field.type === "weekday_multi_mode") {
      return (
        <WeekdayMultiModeInput
          field={field}
          inputName={inputName}
          value={value}
          onChange={onChange}
          error={error}
          tr={tr}
          relevantDays={relevantDays}
          companionNote={companionNote}
          onCompanionNoteChange={onCompanionNoteChange}
          companionNoteError={companionNoteError}
          companionNoteName={companionNoteName}
          singleMode={singleMode}
        />
      );
    }
    if (field.type === "file") {
      return (
        <FileUploadInput
          field={field}
          inputName={inputName}
          value={value}
          onChange={onChange}
          error={error}
          tr={tr}
        />
      );
    }
    if (field.type === "contact_list") {
      return (
        <ContactListInput
          field={field}
          inputName={inputName}
          value={value}
          onChange={onChange}
          error={error}
          tr={tr}
        />
      );
    }

    if (field.type === "date")
      return (
        <CustomDateFieldInput
          field={field}
          inputName={inputName}
          value={value}
          onChange={onChange}
          error={error}
        />
      );
    return null;
  };
  return (
    <SharedCustomFieldInput
      field={field}
      inputName={inputName}
      value={value}
      onChange={onChange}
      error={error}
      tr={tr}
      renderStructured={renderStructured}
    />
  );
}
