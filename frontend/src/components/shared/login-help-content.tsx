interface LoginHelpContentProps {
  readonly accountType: string;
  readonly emailLabel: string;
  readonly passwordLabel: string;
}

export function LoginHelpContent({
  accountType,
  emailLabel,
  passwordLabel,
}: LoginHelpContentProps) {
  return (
    <div>
      <p>
        Melden Sie sich mit Ihrem <strong>{accountType}</strong> an:
      </p>
      <ul className="mt-3 space-y-2">
        <li>
          • <strong>E-Mail:</strong> {emailLabel}
        </li>
        <li>
          • <strong>Passwort:</strong> {passwordLabel}
        </li>
      </ul>
      <p className="mt-4">
        <strong>Probleme beim Anmelden?</strong>
      </p>
      <ul className="mt-2 space-y-1 text-sm">
        <li>
          • Überprüfen Sie Ihre <strong>Internetverbindung</strong>
        </li>
        <li>
          • Stellen Sie sicher, dass <strong>Caps Lock</strong> deaktiviert ist
        </li>
        <li>
          • Kontaktieren Sie den <strong>Support</strong> bei anhaltenden
          Problemen
        </li>
      </ul>
    </div>
  );
}
