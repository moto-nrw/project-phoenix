package domain

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The Träger scope (ADR 0218): the stored recipient of a Träger and the
// rules a recipient must meet before anything is saved.

// Recipient field names in form order; the contract publishes the same.
const (
	FieldHost        = "host"
	FieldPort        = "port"
	FieldUsername    = "username"
	FieldPassword    = "password"
	FieldDirectory   = "remote_directory"
	FieldFingerprint = "host_key_fingerprint"
)

// DefaultPort is the protocol's own standard, the only preset value.
const DefaultPort = 22

// The value shapes of the school settings (settings/internal/defaults/sftp.go),
// so a Träger enters what a school enters.
var (
	hostPattern     = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.\-]{0,251}[A-Za-z0-9])?$`)
	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._@\-]{1,64}$`)
)

// maxPasswordBytes bounds a password; an SFTP password is short.
const maxPasswordBytes = 256

// StoredRecipient is the recipient row of a Träger. SealedPassword is the
// bank-ring ciphertext; the plain password never lives in this value.
type StoredRecipient struct {
	OrganizationID     int64
	Enabled            bool
	Host               string
	Port               int
	Username           string
	SealedPassword     []byte
	RemoteDirectory    string
	HostKeyFingerprint string
	UpdatedAt          time.Time
}

// RecipientValues is a recipient as the Träger entered it, before sealing.
// Password is nil when the stored one stays.
type RecipientValues struct {
	Enabled            bool
	Host               string
	Port               int
	Username           string
	Password           *string
	RemoteDirectory    string
	HostKeyFingerprint string
}

// Normalize trims what is always a copy-paste artefact. The password is
// never trimmed: a different secret would fail as a wrong password.
func (v RecipientValues) Normalize() RecipientValues {
	v.Host = strings.TrimSpace(v.Host)
	v.Username = strings.TrimSpace(v.Username)
	v.RemoteDirectory = strings.TrimSpace(v.RemoteDirectory)
	v.HostKeyFingerprint = strings.TrimSpace(v.HostKeyFingerprint)
	return v
}

// InvalidFields lists the entered values that are malformed, in form order.
// An empty value is not malformed; it only keeps the recipient not ready.
func (v RecipientValues) InvalidFields() []string {
	var invalid []string
	if v.Host != "" && !hostPattern.MatchString(v.Host) {
		invalid = append(invalid, FieldHost)
	}
	if !ValidPort(v.Port) {
		invalid = append(invalid, FieldPort)
	}
	if v.Username != "" && !usernamePattern.MatchString(v.Username) {
		invalid = append(invalid, FieldUsername)
	}
	if v.Password != nil && (*v.Password == "" || len(*v.Password) > maxPasswordBytes || strings.ContainsAny(*v.Password, "\r\n\x00")) {
		invalid = append(invalid, FieldPassword)
	}
	if v.RemoteDirectory != "" && (!ValidRemoteDirectory(v.RemoteDirectory) || len(v.RemoteDirectory) > 512 ||
		strings.ContainsAny(v.RemoteDirectory, "\\\r\n\x00")) {
		invalid = append(invalid, FieldDirectory)
	}
	if v.HostKeyFingerprint != "" && !ValidFingerprint(v.HostKeyFingerprint) {
		invalid = append(invalid, FieldFingerprint)
	}
	return invalid
}

// MissingFields lists what blocks a handover, in form order. passwordOpens
// says whether a stored password exists and still opens.
func (r StoredRecipient) MissingFields(passwordOpens bool) []string {
	missing := []string{}
	if r.Host == "" {
		missing = append(missing, FieldHost)
	}
	if !ValidPort(r.Port) {
		missing = append(missing, FieldPort)
	}
	if r.Username == "" {
		missing = append(missing, FieldUsername)
	}
	if !passwordOpens {
		missing = append(missing, FieldPassword)
	}
	if !ValidRemoteDirectory(r.RemoteDirectory) {
		missing = append(missing, FieldDirectory)
	}
	if !ValidFingerprint(r.HostKeyFingerprint) {
		missing = append(missing, FieldFingerprint)
	}
	return missing
}

// PasswordContext binds a sealed password to its column and its Träger: a
// ciphertext copied to another Träger or another column does not open.
func PasswordContext(organizationID int64) []byte {
	return []byte("config.traeger_export_recipients.password|" + strconv.FormatInt(organizationID, 10))
}

// TraegerJournalEntry is one recorded handover of a Träger. Like the school
// journal it holds no credential and no file content.
type TraegerJournalEntry struct {
	OrganizationID  int64
	ActorAccountID  int64
	Kind            string
	Format          string
	Filename        string
	ByteSize        int64
	TargetHost      string
	TargetPort      int
	TargetDirectory string
	Success         bool
	Reason          string
}

// TraegerJournalRow is a journal row as read back.
type TraegerJournalRow struct {
	ID              int64
	CreatedAt       time.Time
	Format          string
	Filename        string
	ByteSize        int64
	TargetHost      string
	TargetDirectory string
	Status          string
	Reason          string
	DispatchState   string
	DispatchExpired bool
}
