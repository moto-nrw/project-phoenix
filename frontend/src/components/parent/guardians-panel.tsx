"use client";

import { type ReactNode, useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Plus } from "lucide-react";
import {
  Check,
  EnvelopeSimple,
  MapPin,
  Note,
  PencilSimple,
  Phone,
  ShieldCheck,
  UserPlus,
  X,
} from "~/components/parent/shell/parent-icons";
import {
  type ChildGuardian,
  type GuardianContactPayload,
  type GuardianRelationshipPayload,
  type RelatedAccount,
  ParentApiError,
  inviteRelatedAccount,
  listChildGuardians,
  listRelatedAccounts,
  removeRelatedAccount,
  updateGuardianContact,
  updateGuardianRelationship,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { GUARDIAN_ROLE_OPTIONS } from "~/lib/guardian-helpers";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Alert } from "~/components/ui/alert";
import { ParentSection } from "~/components/parent/shell/parent-section";

const logger = createLogger({ component: "GuardiansPanel" });

const RELATIONSHIP_LABELS: Record<string, string> = {
  parent: "Elternteil",
  guardian: "Erziehungsberechtigt",
  relative: "Verwandt",
  other: "Weitere Person",
};

const PHONE_TYPE_LABELS: Record<string, string> = {
  mobile: "Mobil",
  home: "Festnetz",
  work: "Geschäftlich",
  other: "Sonstige",
};

const PHONE_TYPES = ["mobile", "home", "work", "other"] as const;
type PhoneType = (typeof PHONE_TYPES)[number];
const RELATIONSHIP_TYPES = ["parent", "guardian", "relative", "other"] as const;
type RelationshipType = (typeof RELATIONSHIP_TYPES)[number];

function isPhoneType(type: string): type is PhoneType {
  return PHONE_TYPES.includes(type as PhoneType);
}

function isRelationshipType(type: string): type is RelationshipType {
  return RELATIONSHIP_TYPES.includes(type as RelationshipType);
}

function initials(first: string, last: string): string {
  return `${first.charAt(0)}${last.charAt(0)}`.toUpperCase() || "?";
}

let nextPhoneDraftId = 0;

function newPhoneDraftId(): string {
  nextPhoneDraftId += 1;
  return `new-phone-${nextPhoneDraftId}`;
}

function resolveGuardianError(
  t: ReturnType<typeof useTranslations<"parentChildDetail">>,
  err: unknown,
  fallbackKey: "guardians.contactSaveError" | "guardians.pickupSaveError",
): string {
  if (err instanceof ParentApiError) {
    switch (err.code) {
      case "guardian_contact_invalid":
        return t("guardians.errors.contactInvalid");
      case "guardian_relationship_invalid":
        return t("guardians.errors.relationshipInvalid");
      case "guardian_email_conflict":
        return t("guardians.errors.emailConflict");
      case "guardian_has_own_account":
        return t("guardians.errors.hasOwnAccount");
      case "guardian_shared_across_families":
        return t("guardians.errors.sharedAcrossFamilies");
      case "guardian_social_worker_managed":
        return t("guardians.errors.socialWorkerManaged");
      case "guardian_role_managed":
        return t("guardians.errors.roleManaged");
      case "guardian_management_disabled":
        return t("guardians.errors.managementDisabled");
      case "guardian_not_linked":
        return t("guardians.errors.notLinked");
      case "guardian_permission_denied":
        return t("guardians.errors.permissionDenied");
      case "guardian_no_change":
        return t("guardians.errors.noChange");
    }
  }
  return t(fallbackKey);
}

interface GuardiansPanelProps {
  readonly studentId: string;
  readonly canInvite: boolean;
  readonly canRemove: boolean;
}

/**
 * "Eltern und Abholberechtigte": ALLE Personen rund um das Kind an einer
 * Stelle.
 *
 * Frueher standen dieselben Menschen zweimal auf der Seite, einmal als
 * "Sorgeberechtigte" mit Kontaktdaten und einmal als "Verknuepfte Konten" mit
 * Zugangsstatus (#2308). Jetzt gibt es eine Liste: Kontakt, Abholrecht und
 * Zugang zur App stehen in derselben Zeile.
 */
export default function GuardiansPanel({
  studentId,
  canInvite,
  canRemove,
}: GuardiansPanelProps) {
  const t = useTranslations("parentChildDetail");
  const [guardians, setGuardians] = useState<ChildGuardian[]>([]);
  const [accounts, setAccounts] = useState<RelatedAccount[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [editingContact, setEditingContact] = useState<ChildGuardian | null>(
    null,
  );
  const [editingPickup, setEditingPickup] = useState<ChildGuardian | null>(
    null,
  );
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmRemoveId, setConfirmRemoveId] = useState<string | null>(null);
  // Bestaetigung fuer den Ausbau eines eingeschraenkten Kontakts (#2172).
  const [upgradePrompt, setUpgradePrompt] = useState<{
    email: string;
    name: string;
    existingRole?: string;
  } | null>(null);
  const [message, setMessage] = useState<{
    kind: "success" | "error";
    text: string;
  } | null>(null);

  const load = useCallback(async () => {
    try {
      setIsLoading(true);
      const [guardianList, accountList] = await Promise.all([
        listChildGuardians(studentId),
        listRelatedAccounts(studentId).catch(() => [] as RelatedAccount[]),
      ]);
      setGuardians(guardianList);
      setAccounts(accountList);
    } catch (err) {
      logger.error("guardians_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMessage({
        kind: "error",
        text: t("guardians.loadError"),
      });
    } finally {
      setIsLoading(false);
    }
  }, [studentId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleSaved = useCallback(
    async (text: string) => {
      setEditingContact(null);
      setEditingPickup(null);
      await load();
      setMessage({ kind: "success", text });
    },
    [load],
  );

  const accountById = new Map(
    accounts.map((account) => [account.guardian_profile_id, account]),
  );
  // Kontakte, zu denen es keine Sorgeberechtigten-Zeile gibt, gehen sonst
  // verloren; sie bekommen eine eigene, schlanke Zeile.
  const orphanAccounts = accounts.filter(
    (account) =>
      !guardians.some(
        (g) => g.guardian_profile_id === account.guardian_profile_id,
      ),
  );

  const handleInvite = async (email: string, confirmRoleUpgrade = false) => {
    const trimmed = email.trim();
    if (!trimmed) return;
    setBusy(true);
    setMessage(null);
    try {
      const result = await inviteRelatedAccount(
        studentId,
        trimmed,
        confirmRoleUpgrade ? { confirmRoleUpgrade: true } : undefined,
      );
      if (result.outcome === "existing_contact_restricted") {
        setUpgradePrompt({
          email: trimmed,
          name: trimmed,
          existingRole: result.existing_role,
        });
        return;
      }
      setUpgradePrompt(null);
      setInviteEmail("");
      setInviteOpen(false);
      await load();
      setMessage({
        kind: "success",
        text:
          result.outcome === "pending_approval"
            ? t("guardians.access.invitePending")
            : result.outcome === "invited"
              ? t("guardians.access.inviteSent", { email: trimmed })
              : t("guardians.access.inviteLinked"),
      });
    } catch (err) {
      logger.error("related_account_invite_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setUpgradePrompt(null);
      setMessage({
        kind: "error",
        text:
          err instanceof ParentApiError &&
          err.code === "guardian_social_worker_managed"
            ? t("guardians.errors.socialWorkerManaged")
            : t("guardians.access.inviteError"),
      });
    } finally {
      setBusy(false);
    }
  };

  const handleRemove = async (guardianProfileId: string) => {
    setBusy(true);
    setMessage(null);
    try {
      await removeRelatedAccount(studentId, guardianProfileId);
      setConfirmRemoveId(null);
      await load();
      setMessage({ kind: "success", text: t("guardians.access.removed") });
    } catch (err) {
      logger.error("related_account_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMessage({ kind: "error", text: t("guardians.access.removeError") });
    } finally {
      setBusy(false);
    }
  };

  return (
    <ParentSection
      title={t("guardians.title")}
      description={t("guardians.description")}
      actions={
        canInvite ? (
          <Button
            type="button"
            variant="outline"
            size="touch"
            onClick={() => {
              setInviteOpen((open) => !open);
              setMessage(null);
            }}
          >
            <UserPlus size={20} className="mr-2 shrink-0" aria-hidden="true" />
            {t("guardians.access.invite")}
          </Button>
        ) : undefined
      }
    >
      {message && (
        <Alert
          type={message.kind === "success" ? "success" : "error"}
          message={message.text}
        />
      )}

      {inviteOpen && canInvite && (
        <div className="flex flex-col gap-2 rounded-xl border border-gray-200 p-3 sm:flex-row">
          <Input
            type="email"
            value={inviteEmail}
            onChange={(event) => setInviteEmail(event.target.value)}
            placeholder={t("guardians.access.emailPlaceholder")}
            aria-label={t("guardians.access.emailLabel")}
            className="flex-1"
          />
          <Button
            type="button"
            size="touch"
            className="shrink-0"
            onClick={() => void handleInvite(inviteEmail)}
            disabled={busy || !inviteEmail.trim()}
          >
            <Check size={20} className="mr-2 shrink-0" aria-hidden="true" />
            {t("guardians.access.send")}
          </Button>
        </div>
      )}

      {isLoading ? (
        <p className="text-[15px] text-gray-500">{t("guardians.loading")}</p>
      ) : guardians.length === 0 && orphanAccounts.length === 0 ? (
        <p className="text-[15px] text-gray-500">{t("guardians.empty")}</p>
      ) : (
        <>
          {guardians.map((g) => (
            <GuardianRow
              key={g.guardian_profile_id}
              guardian={g}
              access={accountById.get(g.guardian_profile_id)}
              canInvite={canInvite}
              canRemove={canRemove}
              busy={busy}
              confirmingRemove={confirmRemoveId === g.guardian_profile_id}
              onEditContact={() => {
                setMessage(null);
                setEditingContact(g);
              }}
              onEditPickup={() => {
                setMessage(null);
                setEditingPickup(g);
              }}
              onGrantAccess={(email, name) => setUpgradePrompt({ email, name })}
              onAskRemove={() => setConfirmRemoveId(g.guardian_profile_id)}
              onCancelRemove={() => setConfirmRemoveId(null)}
              onConfirmRemove={() => void handleRemove(g.guardian_profile_id)}
            />
          ))}
          {orphanAccounts.map((account) => (
            <AccountOnlyRow
              key={account.guardian_profile_id}
              account={account}
            />
          ))}
        </>
      )}

      {editingContact && (
        <ContactModal
          studentId={studentId}
          guardian={editingContact}
          onClose={() => setEditingContact(null)}
          onSaved={() => handleSaved(t("guardians.contactSaved"))}
        />
      )}
      {editingPickup && (
        <PickupModal
          studentId={studentId}
          guardian={editingPickup}
          onClose={() => setEditingPickup(null)}
          onSaved={() => handleSaved(t("guardians.pickupSaved"))}
        />
      )}

      <ConfirmationModal
        isOpen={upgradePrompt !== null}
        onClose={() => setUpgradePrompt(null)}
        onConfirm={() => {
          if (upgradePrompt) void handleInvite(upgradePrompt.email, true);
        }}
        title={t("guardians.access.grantTitle")}
        confirmText={t("guardians.access.grant")}
        cancelText={t("guardians.cancel")}
        isConfirmLoading={busy}
      >
        <p className="text-[15px] text-gray-600">
          {upgradePrompt
            ? guardianRoleLabel(upgradePrompt.existingRole)
              ? t("guardians.access.grantBodyWithRole", {
                  name: upgradePrompt.name,
                  role: guardianRoleLabel(upgradePrompt.existingRole) ?? "",
                })
              : t("guardians.access.grantBody", { name: upgradePrompt.name })
            : ""}
        </p>
      </ConfirmationModal>
    </ParentSection>
  );
}

function guardianRoleLabel(role: string | undefined): string | null {
  return (
    GUARDIAN_ROLE_OPTIONS.find((option) => option.value === role)?.label ?? null
  );
}

/** Ein Kontakt, den es nur als Konto gibt, ohne Sorgeberechtigten-Zeile. */
function AccountOnlyRow({ account }: Readonly<{ account: RelatedAccount }>) {
  const t = useTranslations("parentChildDetail");
  const name =
    `${account.first_name} ${account.last_name}`.trim() ||
    (account.email ?? "");
  return (
    <div className="flex items-center gap-3 rounded-xl border border-gray-200 p-3">
      <span className="flex size-11 shrink-0 items-center justify-center rounded-full bg-gray-100 text-[15px] font-semibold text-gray-700">
        {initials(account.first_name, account.last_name)}
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-[17px] font-semibold text-gray-900">
          {name}
        </p>
        <p className="truncate text-[15px] text-gray-600">
          {t(`guardians.access.status.${account.status}`)}
        </p>
      </div>
    </div>
  );
}

function GuardianRow({
  guardian: g,
  access,
  canInvite,
  canRemove,
  busy,
  confirmingRemove,
  onEditContact,
  onEditPickup,
  onGrantAccess,
  onAskRemove,
  onCancelRemove,
  onConfirmRemove,
}: Readonly<{
  guardian: ChildGuardian;
  access?: RelatedAccount;
  canInvite: boolean;
  canRemove: boolean;
  busy: boolean;
  confirmingRemove: boolean;
  onEditContact: () => void;
  onEditPickup: () => void;
  onGrantAccess: (email: string, name: string) => void;
  onAskRemove: () => void;
  onCancelRemove: () => void;
  onConfirmRemove: () => void;
}>) {
  const t = useTranslations("parentChildDetail");
  const name = `${g.first_name} ${g.last_name}`.trim() || "Kontakt";
  const relationshipLabel = isRelationshipType(g.relationship_type)
    ? t(`guardians.relationships.${g.relationship_type}`)
    : (RELATIONSHIP_LABELS[g.relationship_type] ??
      t("guardians.relationships.contact"));
  const hasContact =
    g.phones.length > 0 ||
    Boolean(g.email) ||
    Boolean(g.address_street ?? g.address_city ?? g.address_postal_code);
  const hasDetails = hasContact || Boolean(g.pickup_notes);
  // Von der Schule verwaltete Kontakte (social_worker) weist das Backend ab,
  // deshalb wird der Ausbau dort gar nicht angeboten (#2172).
  const grantable =
    canInvite &&
    access?.status === "active_no_access" &&
    access.guardian_role !== "social_worker" &&
    Boolean(access.email);
  const removable =
    canRemove &&
    access !== undefined &&
    !access.is_primary &&
    !access.is_self &&
    access.status !== "no_account";
  return (
    <div className="rounded-xl border border-gray-200 p-4">
      <div className="flex items-start gap-3">
        <span className="flex size-11 shrink-0 items-center justify-center rounded-full bg-gray-100 text-[15px] font-semibold text-gray-700">
          {initials(g.first_name, g.last_name)}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <p className="text-[17px] font-semibold text-gray-900">{name}</p>
            <span className="text-[15px] text-gray-500">
              {relationshipLabel}
            </span>
            {g.is_primary && (
              <span className="text-[15px] font-medium text-gray-500">
                ({t("guardians.badges.primary")})
              </span>
            )}
          </div>
          {/* Der Zugang zur App gehoert an die Person, nicht in eine zweite
              Liste derselben Menschen. */}
          {access && (
            <p className="mt-1 text-[15px] text-gray-600">
              {t(`guardians.access.status.${access.status}`)}
            </p>
          )}
          {/* Neutral text chips, not colored dots: "darf abholen" and
              "Notfallkontakt" are facts about a person, not alert states, and a
              green/amber dot per contact turned the list into a traffic light. */}
          {(g.can_pickup || g.is_emergency_contact) && (
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              {g.can_pickup && (
                <span className="rounded-full bg-gray-100 px-2.5 py-0.5 text-[15px] font-medium text-gray-700">
                  {t("guardians.badges.canPickup")}
                </span>
              )}
              {g.is_emergency_contact && (
                <span className="rounded-full bg-gray-100 px-2.5 py-0.5 text-[15px] font-medium text-gray-700">
                  {t("guardians.badges.emergency")}
                </span>
              )}
            </div>
          )}
          {hasDetails && (
            <div className="mt-2 space-y-1.5 text-[15px] text-gray-600">
              {hasContact && (
                <>
                  {g.phones.map((p) => (
                    <ContactLine
                      key={`${p.phone_number}-${p.phone_type}`}
                      icon={<Phone size={16} aria-hidden="true" />}
                      text={[
                        p.phone_number,
                        p.label,
                        p.phone_type && isPhoneType(p.phone_type)
                          ? t(`guardians.phoneTypes.${p.phone_type}`)
                          : p.phone_type
                            ? (PHONE_TYPE_LABELS[p.phone_type] ?? p.phone_type)
                            : null,
                      ]
                        .filter(Boolean)
                        .join(" · ")}
                    />
                  ))}
                  {g.email && (
                    <ContactLine
                      icon={<EnvelopeSimple size={16} aria-hidden="true" />}
                      text={g.email}
                    />
                  )}
                  {(g.address_street ??
                    g.address_city ??
                    g.address_postal_code) && (
                    <ContactLine
                      icon={<MapPin size={16} aria-hidden="true" />}
                      text={[
                        g.address_street,
                        [g.address_postal_code, g.address_city]
                          .filter(Boolean)
                          .join(" "),
                      ]
                        .filter((s) => s && s.trim())
                        .join(", ")}
                    />
                  )}
                </>
              )}
              {g.pickup_notes && (
                <ContactLine
                  icon={<ShieldCheck size={16} aria-hidden="true" />}
                  text={g.pickup_notes}
                />
              )}
            </div>
          )}
        </div>
        {(g.can_edit_contact || g.can_manage_pickup) && (
          <div className="flex shrink-0 items-center gap-1">
            {g.can_edit_contact && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={onEditContact}
                title={t("guardians.editContact")}
                aria-label={t("guardians.editContact")}
                className="text-gray-400"
              >
                <PencilSimple size={20} aria-hidden="true" />
              </Button>
            )}
            {/* The pickup/relationship modal carries two capabilities: the
                safety-critical can_pickup / emergency flags (pickup.manage) and
                the per-child pickup note. The note follows contact editability —
                a parent may edit it only for a guardian whose contact they may
                also edit (can_edit_contact) — so surface the action when the
                caller may edit the note OR manage pickup. A note-only editor
                (e.g. their own row, where flags are never theirs to set) still
                reaches the note. The label/icon reflect which capability applies. */}
            {(g.can_edit_contact || g.can_manage_pickup) && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={onEditPickup}
                title={
                  g.can_manage_pickup
                    ? t("guardians.managePickup")
                    : t("guardians.pickupNotes")
                }
                aria-label={
                  g.can_manage_pickup
                    ? t("guardians.managePickup")
                    : t("guardians.pickupNotes")
                }
                className="text-gray-400"
              >
                {g.can_manage_pickup ? (
                  <ShieldCheck size={20} aria-hidden="true" />
                ) : (
                  <Note size={20} aria-hidden="true" />
                )}
              </Button>
            )}
          </div>
        )}
      </div>

      {/* Zugang zur App: gewaehren oder entziehen. Steht bei der Person,
          nicht in einer zweiten Liste (#2308). */}
      {(grantable || removable) && (
        <div className="mt-3 flex flex-wrap gap-2 border-t border-gray-100 pt-3">
          {grantable && (
            <Button
              type="button"
              variant="outline"
              size="touch"
              disabled={busy}
              onClick={() => onGrantAccess(access?.email ?? "", name)}
            >
              {t("guardians.access.grant")}
            </Button>
          )}
          {removable &&
            (confirmingRemove ? (
              <>
                <Button
                  type="button"
                  variant="danger"
                  size="touch"
                  disabled={busy}
                  onClick={onConfirmRemove}
                >
                  {t("guardians.access.removeConfirm")}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="touch"
                  onClick={onCancelRemove}
                >
                  {t("guardians.cancel")}
                </Button>
              </>
            ) : (
              <Button
                type="button"
                variant="outline_danger"
                size="touch"
                onClick={onAskRemove}
              >
                <X size={20} className="mr-2 shrink-0" aria-hidden="true" />
                {t("guardians.access.remove")}
              </Button>
            ))}
        </div>
      )}
    </div>
  );
}

function ContactLine({
  icon,
  text,
}: Readonly<{ icon: ReactNode; text: string }>) {
  return (
    <p className="flex items-center gap-1.5 break-words">
      <span className="shrink-0 text-gray-400">{icon}</span>
      {text}
    </p>
  );
}

interface PhoneDraft {
  id: string;
  phone_number: string;
  phone_type: string;
  label: string | null;
  is_primary: boolean;
}

function ContactModal({
  studentId,
  guardian: g,
  onClose,
  onSaved,
}: Readonly<{
  studentId: string;
  guardian: ChildGuardian;
  onClose: () => void;
  onSaved: () => void;
}>) {
  const t = useTranslations("parentChildDetail");
  const [firstName, setFirstName] = useState(g.first_name);
  const [lastName, setLastName] = useState(g.last_name);
  const [email, setEmail] = useState(g.email ?? "");
  const [street, setStreet] = useState(g.address_street ?? "");
  const [city, setCity] = useState(g.address_city ?? "");
  const [postal, setPostal] = useState(g.address_postal_code ?? "");
  const [phones, setPhones] = useState<PhoneDraft[]>(
    g.phones.length > 0
      ? g.phones.map((p) => ({
          id: `${g.guardian_profile_id}-${p.phone_number}-${p.phone_type}`,
          phone_number: p.phone_number,
          phone_type: p.phone_type || "mobile",
          label: p.label ?? null,
          is_primary: p.is_primary,
        }))
      : [
          {
            id: newPhoneDraftId(),
            phone_number: "",
            phone_type: "mobile",
            label: null,
            is_primary: true,
          },
        ],
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const setPhone = (index: number, patch: Partial<PhoneDraft>) =>
    setPhones((prev) =>
      prev.map((p, i) => (i === index ? { ...p, ...patch } : p)),
    );

  const handleSave = async () => {
    if (!firstName.trim() || !lastName.trim()) {
      setError(t("guardians.nameRequired"));
      return;
    }
    setBusy(true);
    setError(null);
    const cleaned = phones
      .map((p) => ({ ...p, phone_number: p.phone_number.trim() }))
      .filter((p) => p.phone_number !== "");
    const primaryIndex = cleaned.findIndex((p) => p.is_primary);
    const fallbackPrimaryIndex = cleaned.length > 0 ? 0 : -1;
    const effectivePrimaryIndex =
      primaryIndex >= 0 ? primaryIndex : fallbackPrimaryIndex;
    const payload: GuardianContactPayload = {
      first_name: firstName.trim(),
      last_name: lastName.trim(),
      email: email.trim() || null,
      address_street: street.trim() || null,
      address_city: city.trim() || null,
      address_postal_code: postal.trim() || null,
      phones: cleaned.map((p, i) => ({
        phone_number: p.phone_number,
        phone_type: p.phone_type,
        label: p.label,
        is_primary: i === effectivePrimaryIndex,
      })),
    };
    try {
      await updateGuardianContact(studentId, g.guardian_profile_id, payload);
      onSaved();
    } catch (err) {
      logger.error("guardian_contact_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(resolveGuardianError(t, err, "guardians.contactSaveError"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("guardians.contactTitle")}
      closeLabel={t("guardians.close")}
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            {t("guardians.cancel")}
          </Button>
          <Button
            type="button"
            size="md"
            disabled={busy}
            onClick={() => void handleSave()}
          >
            {t("guardians.save")}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {error && (
          <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border p-3 text-sm">
            {error}
          </div>
        )}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Input
            label={t("guardians.firstName")}
            value={firstName}
            onChange={(e) => setFirstName(e.target.value)}
          />
          <Input
            label={t("guardians.lastName")}
            value={lastName}
            onChange={(e) => setLastName(e.target.value)}
          />
        </div>
        <Input
          label={t("guardians.email")}
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
        <Input
          label={t("guardians.street")}
          value={street}
          onChange={(e) => setStreet(e.target.value)}
        />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-[8rem_minmax(0,1fr)]">
          <Input
            label={t("guardians.postalCode")}
            value={postal}
            onChange={(e) => setPostal(e.target.value)}
          />
          <Input
            label={t("guardians.city")}
            value={city}
            onChange={(e) => setCity(e.target.value)}
          />
        </div>

        <div>
          <p className="mb-2 text-sm font-medium text-gray-700">
            {t("guardians.phoneNumbers")}
          </p>
          <div className="space-y-2">
            {phones.map((p, i) => (
              <div key={p.id} className="flex items-center gap-2">
                <div className="min-w-0 flex-1">
                  <Input
                    type="tel"
                    value={p.phone_number}
                    onChange={(e) =>
                      setPhone(i, { phone_number: e.target.value })
                    }
                    placeholder={t("guardians.phonePlaceholder")}
                    controlSize="compact"
                  />
                </div>
                <div className="w-32 shrink-0">
                  <CustomSelect
                    value={p.phone_type}
                    options={PHONE_TYPES.map((type) => ({
                      value: type,
                      label: t(`guardians.phoneTypes.${type}`),
                    }))}
                    onChange={(next) => setPhone(i, { phone_type: next })}
                    ariaLabel={t("guardians.phoneTypeLabel")}
                  />
                </div>
                {phones.length > 1 && (
                  <button
                    type="button"
                    onClick={() =>
                      setPhones((prev) => prev.filter((_, idx) => idx !== i))
                    }
                    className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
                    aria-label={t("guardians.removePhone")}
                  >
                    <X className="h-4 w-4" />
                  </button>
                )}
              </div>
            ))}
          </div>
          {phones.length < 5 && (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              className="mt-2"
              onClick={() =>
                setPhones((prev) => [
                  ...prev,
                  {
                    id: newPhoneDraftId(),
                    phone_number: "",
                    phone_type: "mobile",
                    label: null,
                    is_primary: false,
                  },
                ])
              }
            >
              <Plus className="h-3.5 w-3.5" aria-hidden="true" />
              {t("guardians.addPhone")}
            </Button>
          )}
        </div>
      </div>
    </Modal>
  );
}

function PickupModal({
  studentId,
  guardian: g,
  onClose,
  onSaved,
}: Readonly<{
  studentId: string;
  guardian: ChildGuardian;
  onClose: () => void;
  onSaved: () => void;
}>) {
  const t = useTranslations("parentChildDetail");
  const [canPickup, setCanPickup] = useState(g.can_pickup);
  const [isEmergency, setIsEmergency] = useState(g.is_emergency_contact);
  const [notes, setNotes] = useState(g.pickup_notes ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async () => {
    setBusy(true);
    setError(null);
    // Send each field only when it actually changed. The backend gates each
    // field group by its own permission: the can_pickup / is_emergency_contact
    // flags require parent_portal.pickup.manage, while pickup_notes requires
    // parent_portal.guardian.edit. Sending an unchanged flag would force the
    // pickup.manage gate even on a notes-only edit, defeating that split.
    //
    // For a CLEARED note send "" (empty string), never null: the backend
    // unmarshals JSON null into a nil *string, which it treats as "field
    // omitted, leave unchanged" — so a null would silently keep the old note (or
    // come back as guardian_no_change). An empty string is a present value the
    // backend trims and stores as NULL, actually clearing the note.
    //
    // The note may ONLY travel when the caller can edit contact: the textarea is
    // rendered only for can_edit_contact, and the backend gates pickup_notes on
    // parent_portal.guardian.edit. Gating the spread on can_edit_contact (and
    // comparing against a TRIMMED original) keeps a pickup-manage-only caller's
    // flag toggle from carrying a phantom pickup_notes — e.g. when legacy/staff
    // data has surrounding whitespace the hidden, untouched note would otherwise
    // "differ" from its trimmed self and trip the guardian.edit gate, failing an
    // otherwise valid flag-only edit (#1667 review).
    const normalizedNotes = notes.trim();
    const originalNotes = (g.pickup_notes ?? "").trim();
    const noteChanged = g.can_edit_contact && normalizedNotes !== originalNotes;
    const payload: GuardianRelationshipPayload = {
      ...(canPickup !== g.can_pickup ? { can_pickup: canPickup } : {}),
      ...(isEmergency !== g.is_emergency_contact
        ? { is_emergency_contact: isEmergency }
        : {}),
      ...(noteChanged ? { pickup_notes: normalizedNotes } : {}),
    };
    // Nothing changed: skip the write (the backend rejects an empty payload
    // with guardian_no_change) and just close.
    if (Object.keys(payload).length === 0) {
      setBusy(false);
      onClose();
      return;
    }
    try {
      await updateGuardianRelationship(
        studentId,
        g.guardian_profile_id,
        payload,
      );
      onSaved();
    } catch (err) {
      logger.error("guardian_pickup_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(resolveGuardianError(t, err, "guardians.pickupSaveError"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={
        g.can_manage_pickup
          ? t("guardians.pickupTitle")
          : t("guardians.pickupNotes")
      }
      closeLabel={t("guardians.close")}
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            {t("guardians.cancel")}
          </Button>
          <Button
            type="button"
            size="md"
            disabled={busy}
            onClick={() => void handleSave()}
          >
            {t("guardians.save")}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {error && (
          <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border p-3 text-sm">
            {error}
          </div>
        )}
        <p className="text-sm text-gray-600">
          {`${g.first_name} ${g.last_name}`.trim()}
        </p>
        {/* The can_pickup / emergency flags are safety-critical authority gated
            by parent_portal.pickup.manage; render them only when the caller may
            manage pickup so the modal never offers a control the backend rejects
            (a note-only editor sees just the note field below). */}
        {g.can_manage_pickup && (
          <>
            <label
              htmlFor="guardian-can-pickup"
              className="flex items-start gap-3"
            >
              <Checkbox
                id="guardian-can-pickup"
                checked={canPickup}
                onChange={(e) => setCanPickup(e.target.checked)}
              />
              <span className="text-sm">
                <span className="font-medium text-gray-900">
                  {t("guardians.canPickupLabel")}
                </span>
                <span className="mt-0.5 block text-gray-500">
                  {t("guardians.canPickupDescription")}
                </span>
              </span>
            </label>
            <label
              htmlFor="guardian-emergency"
              className="flex items-start gap-3"
            >
              <Checkbox
                id="guardian-emergency"
                checked={isEmergency}
                onChange={(e) => setIsEmergency(e.target.checked)}
              />
              <span className="text-sm">
                <span className="font-medium text-gray-900">
                  {t("guardians.emergencyLabel")}
                </span>
                <span className="mt-0.5 block text-gray-500">
                  {t("guardians.emergencyDescription")}
                </span>
              </span>
            </label>
          </>
        )}
        {/* The note follows contact editability: a parent may edit it only for a
            guardian whose contact they may also edit, so gate it on
            can_edit_contact. A pickup-manage-only caller sees just the flags
            above; the backend rejects a note edit on a contact-locked guardian. */}
        {g.can_edit_contact && (
          <div>
            <label
              htmlFor="pickup-notes"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              {t("guardians.pickupNotes")}
            </label>
            <textarea
              id="pickup-notes"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={3}
              maxLength={500}
              placeholder={t("guardians.pickupNotesPlaceholder")}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
            />
          </div>
        )}
      </div>
    </Modal>
  );
}
