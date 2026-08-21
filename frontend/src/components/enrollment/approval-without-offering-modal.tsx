import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
}

export function ApprovalWithoutOfferingModal({
  isOpen,
  onClose,
  onConfirm,
}: Props) {
  return (
    <ConfirmationModal
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Anmeldung bestätigen"
      confirmText="Trotzdem bestätigen"
    >
      <Alert
        type="warning"
        message="Für dieses Kind ist kein Betreuungsangebot gebucht. Das Kind wird trotzdem in die OGS aufgenommen."
      />
    </ConfirmationModal>
  );
}
