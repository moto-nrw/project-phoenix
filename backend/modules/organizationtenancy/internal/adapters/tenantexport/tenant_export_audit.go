package tenantexport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// What the Träger package adds to the catalog export (ADR 0187): the
// operator audit of each run and the PAIN.008 files of the submitted SEPA
// messages. The rows, with their opened bank and health values, are the
// catalog export of ADR 0184/0185.

// exportOrganization records the attempt of a Träger package before
// anything is read or written, then its outcome. A failed run removes its
// partial files, so no readable package stays without its record (#4720);
// the failure record says whether the removal succeeded.
func (e *tenantExporter) exportOrganization(ctx context.Context, options organizationtenancy.TenantExportOptions) error {
	if e.audit == nil || e.sepaFiles == nil {
		return errors.New("tenant export: a Träger package needs its PAIN.008 files and its operator audit")
	}
	if options.OperatorID <= 0 {
		return errors.New("tenant export: the operator of a Träger package is required for its audit")
	}
	attempt := organizationtenancy.TenantExportAttempt{OperatorID: options.OperatorID, OrganizationID: options.OrganizationID,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := e.audit.BeginTenantExport(ctx, attempt); err != nil {
		return fmt.Errorf("tenant export: attempt not recorded, nothing was read: %w", err)
	}
	step, created, err := e.run(ctx, options, func(record organizationtenancy.TenantExportRecord) error {
		record.StartedAt = attempt.StartedAt
		if err := e.audit.RecordTenantExport(ctx, record); err != nil {
			return errors.New("tenant export: operator audit unavailable; the export is not complete")
		}
		return nil
	})
	if err == nil {
		return nil
	}
	failure := organizationtenancy.TenantExportRecord{OperatorID: options.OperatorID, OrganizationID: options.OrganizationID,
		Outcome: organizationtenancy.TenantExportFailed, StartedAt: attempt.StartedAt, ErrorClass: step}
	if created {
		failure.PartialFilesRemoved = os.RemoveAll(options.Output) == nil
	}
	if auditErr := e.audit.RecordTenantExport(ctx, failure); auditErr != nil {
		return errors.Join(err, errors.New("tenant export: the failure is not in the operator audit"))
	}
	return err
}

// sepaDebitFileDirectory holds the PAIN.008 files of a Träger package.
const sepaDebitFileDirectory = "SEPA-LASTSCHRIFTDATEIEN"

const sepaDebitFileNotice = "PAIN.008-Dateien der eingereichten Lastschriften mit vollständigen Bankdaten (IBAN, Kontoinhaber); je Datei Original oder rekonstruiert."

// sepaDebitFileReadme is the head of LIESMICH.txt in the directory;
// sepaDebitFileReadmeFiles adds the origin of each file.
const sepaDebitFileReadme = `ACHTUNG: Dieser Ordner enthält die SEPA-Lastschriftdateien (PAIN.008) dieses
Trägers mit vollständigen Bankdaten der Familien: IBAN und Kontoinhaber.

Die Mandate und Lastschriften selbst stehen mit IBAN und Kontoinhaber in
parent_billing.sepa_mandates.jsonl und parent_billing.sepa_debits.jsonl.

Je eingereichte Lastschriftdatei liegt hier die Datei mit derselben
Nachrichten-ID. "Original" ist die bei moto gespeicherte Datei des letzten
Downloads vor der Einreichung. "rekonstruiert" ist aus den gespeicherten
Lastschriften neu erzeugt; der Grund steht bei der Datei.
`

func sepaDebitFileReadmeFiles(files []organizationtenancy.ExportSEPADebitFileOrigin) []byte {
	var readme strings.Builder
	readme.WriteString(sepaDebitFileReadme)
	if len(files) == 0 {
		readme.WriteString("\nKeine eingereichte Lastschriftdatei.\n")
		return []byte(readme.String())
	}
	readme.WriteString("\nDateien:\n")
	for _, file := range files {
		readme.WriteString("\n- " + path.Base(file.Path) + ": " + file.Origin + "\n  " + file.Reason + "\n")
	}
	return []byte(readme.String())
}

// writeSEPADebitFiles writes the PAIN.008 files Parent Billing answers for
// this Träger into their own directory and checksums each in the manifest.
// They are read after the catalog snapshot, in a Träger transaction of the
// owner, so the runbook runs the package with every writer stopped: the
// Träger is deactivated, and nothing changes in between.
func (r *exportRun) writeSEPADebitFiles(ctx context.Context, owner organizationtenancy.TenantExportSEPADebitFiles) error {
	files, err := owner.SEPADebitFiles(ctx, r.options.OrganizationID)
	if err != nil {
		return fmt.Errorf("tenant export: PAIN.008 files unavailable: %w", err)
	}
	if err := os.Mkdir(filepath.Join(r.options.Output, sepaDebitFileDirectory), 0o700); err != nil {
		return errors.New("tenant export: cannot create the PAIN.008 directory")
	}
	origins := make([]organizationtenancy.ExportSEPADebitFileOrigin, 0, len(files))
	for _, file := range files {
		// One plain file name per message: never a path beside or above
		// the directory, never the notice.
		if file.Name == "" || path.Base(file.Name) != file.Name || file.Name == ".." || file.Name == "LIESMICH.txt" ||
			!strings.HasSuffix(file.Name, ".xml") || file.Origin == "" {
			return errors.New("tenant export: invalid PAIN.008 file")
		}
		name := path.Join(sepaDebitFileDirectory, file.Name)
		if err := r.writeArtifact(name, file.Data); err != nil {
			return err
		}
		origins = append(origins, organizationtenancy.ExportSEPADebitFileOrigin{Path: name, Origin: file.Origin, Reason: file.Reason})
	}
	if err := r.writeArtifact(path.Join(sepaDebitFileDirectory, "LIESMICH.txt"), sepaDebitFileReadmeFiles(origins)); err != nil {
		return err
	}
	r.manifest.SEPADebitFiles = &organizationtenancy.ExportSEPADebitFiles{Directory: sepaDebitFileDirectory, Notice: sepaDebitFileNotice, Files: origins}
	return nil
}

// auditRecord names the completed run for the operator audit: counts, the
// file origins and the manifest checksum, no contents, no local path.
func (r *exportRun) auditRecord(checksum string) organizationtenancy.TenantExportRecord {
	return organizationtenancy.TenantExportRecord{
		OperatorID: r.options.OperatorID, OrganizationID: r.options.OrganizationID,
		Outcome: organizationtenancy.TenantExportCompleted, ManifestSHA256: checksum,
		ExportedAt: r.manifest.ExportedAt, Tables: len(r.manifest.Tables), Artifacts: len(r.manifest.Artifacts),
		SEPADebitFiles: r.manifest.SEPADebitFiles,
	}
}
