import { Children, cloneElement, isValidElement } from "react";
import { Inbox, Plus } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState as UIEmptyState } from "~/components/ui/empty-state";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { SwitchRow } from "~/components/ui/switch";

/**
 * Label, control and its field error (#2519) for a control without a `label`
 * prop of its own (`CustomSelect`). A text field uses `Input` with `label`,
 * `required` and `error` directly. With `error` the first child element (the
 * control) gets `aria-invalid` and `aria-describedby`, so a screen reader
 * reads the hint with the field.
 */
export function FormField({
  label,
  htmlFor,
  required,
  error,
  children,
}: {
  readonly label: string;
  readonly htmlFor: string;
  readonly required?: boolean;
  readonly error?: string;
  readonly children: React.ReactNode;
}) {
  const errorId = `${htmlFor}-error`;
  // The control is the first element child; a hint after it stays as is.
  let marked = false;
  const control = error
    ? Children.map(children, (child) => {
        if (marked || !isValidElement<Record<string, unknown>>(child)) {
          return child;
        }
        marked = true;
        // CustomSelect takes its ARIA state as named props.
        return child.type === CustomSelect
          ? cloneElement(child, { invalid: true, ariaDescribedBy: errorId })
          : cloneElement(child, {
              "aria-invalid": true,
              "aria-describedby": errorId,
            });
      })
    : children;
  return (
    <div>
      <FieldLabel id={`${htmlFor}-label`} htmlFor={htmlFor} required={required}>
        {label}
      </FieldLabel>
      {control}
      <FieldErrorHint id={errorId} message={error} />
    </div>
  );
}

export function FieldWarning({ message }: { readonly message: string }) {
  return (
    <Alert
      type="warning"
      size="compact"
      announce="polite"
      message={message}
      className="mt-1"
    />
  );
}

export function DeliveryStatusBadge({ status }: { readonly status: string }) {
  if (status === "pending") {
    return (
      <StatusColorBadge
        label="Ausstehend"
        color={LOCATION_COLORS.WARNING}
        compact
      />
    );
  }
  const badges: Record<string, { label: string; tone: StatusBadgeTone }> = {
    sent: { label: "Gesendet", tone: "green" },
    failed: { label: "Fehlgeschlagen", tone: "red" },
  };
  const badge: { label: string; tone: StatusBadgeTone } = Object.hasOwn(
    badges,
    status,
  )
    ? badges[status]!
    : { label: status, tone: "gray" };
  return <StatusBadge tone={badge.tone} compact label={badge.label} />;
}

export function ProvisioningEmptyState({
  title,
  description,
  buttonLabel,
  onAction,
}: {
  readonly title: string;
  readonly description: string;
  readonly buttonLabel: string;
  readonly onAction: () => void;
}) {
  return (
    <UIEmptyState
      icon={
        <Inbox className="h-12 w-12" strokeWidth={1.5} aria-hidden="true" />
      }
      title={title}
      description={description}
      action={
        <Button type="button" variant="primary" size="md" onClick={onAction}>
          {buttonLabel}
        </Button>
      }
    />
  );
}

export function SimpleEmptyState({
  title,
  description,
}: {
  readonly title: string;
  readonly description: string;
}) {
  return <UIEmptyState title={title} description={description} />;
}

export function SelectWithChevron({
  children,
  value,
  onChange,
  id,
  name,
  disabled,
  required,
  "aria-label": ariaLabel,
  "aria-invalid": ariaInvalid,
  "aria-describedby": ariaDescribedBy,
}: React.SelectHTMLAttributes<HTMLSelectElement> & {
  readonly children: React.ReactNode;
}) {
  const options = Children.toArray(children)
    .filter(isValidElement)
    .map((child) => {
      const optionProps = (
        child as React.ReactElement<
          React.OptionHTMLAttributes<HTMLOptionElement>
        >
      ).props;
      return {
        value: optionProps.value != null ? String(optionProps.value) : "",
        label:
          typeof optionProps.children === "string"
            ? optionProps.children
            : String(optionProps.children ?? ""),
        disabled: optionProps.disabled,
      };
    });

  // Without an explicit aria-label, name the field (trigger AND popup listbox)
  // from the associated FormField <label>, which carries id `${id}-label`.
  // CustomSelect forwards ariaLabelledBy to the popup, which cannot be reached
  // by the label's htmlFor the way the trigger button is.
  const ariaLabelledBy = !ariaLabel && id ? `${id}-label` : undefined;

  return (
    <CustomSelect
      id={id}
      name={name}
      value={value != null ? String(value) : ""}
      options={options}
      onChange={(next) =>
        onChange?.({
          target: { value: next },
        } as unknown as React.ChangeEvent<HTMLSelectElement>)
      }
      disabled={disabled}
      required={required}
      ariaLabel={ariaLabel}
      ariaLabelledBy={ariaLabelledBy}
      ariaDescribedBy={ariaDescribedBy}
      invalid={ariaInvalid === true || ariaInvalid === "true"}
    />
  );
}

export function PlusIcon() {
  return <Plus className="h-5 w-5" strokeWidth={2} aria-hidden="true" />;
}

export function VisibilityToggle({
  hidden,
  onToggle,
}: {
  readonly hidden: boolean;
  readonly onToggle: () => void;
}) {
  return (
    <div className="border-t border-gray-100 pt-4">
      <p className="mb-3 text-xs font-medium text-gray-500 uppercase">
        Sichtbarkeit
      </p>
      <SwitchRow
        label={hidden ? "Verborgen" : "Sichtbar"}
        description={
          hidden
            ? "Schule wird nicht auf der Startseite angezeigt, ist aber über den Direktlink erreichbar."
            : "Schule wird auf der Startseite im Schulwähler angezeigt."
        }
        checked={!hidden}
        onCheckedChange={onToggle}
        ariaLabel="Sichtbarkeit umschalten"
      />
    </div>
  );
}
