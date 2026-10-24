package tenantexport

import (
	"encoding/json"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

func personRelationships(model organizationtenancy.ExportModelTable, catalog organizationtenancy.DataLifecycleCatalog) []organizationtenancy.ExportRelationship {
	relations := append([]organizationtenancy.ExportRelationship(nil), model.Relationships...)
	for _, table := range catalog.Tables {
		if table.Name != model.Name {
			continue
		}
		for column, target := range table.PersonKeys {
			at := strings.LastIndex(target, ".")
			if at > 0 {
				relations = append(relations, organizationtenancy.ExportRelationship{Column: column, TargetTable: target[:at], TargetColumn: target[at+1:]})
			}
		}
	}
	// Historical audit IDs intentionally outlive their FK targets. Their
	// meaning is defined by Audit Platform's records, NOT guessed from values.
	if strings.HasPrefix(model.Name, "audit.") {
		for column, target := range map[string]string{
			"student_id": "users.student_profiles", "matched_student_id": "users.student_profiles",
			"guardian_profile_id": "users.guardian_profiles", "staff_id": "users.staff_school_memberships",
			"target_staff_id": "users.staff_school_memberships",
			"account_id":      "auth.accounts", "actor_account_id": "auth.accounts",
			"request_id": "enrollment.requests",
		} {
			relations = append(relations, organizationtenancy.ExportRelationship{Column: column, TargetTable: target, TargetColumn: "id"})
		}
	}
	return personRecordRelationships(relations)
}

func personRecordRelationships(relations []organizationtenancy.ExportRelationship) []organizationtenancy.ExportRelationship {
	var records []organizationtenancy.ExportRelationship
	seen := map[organizationtenancy.ExportRelationship]bool{}
	for _, rel := range relations {
		// The exporter already enforces the school boundary. Treating one
		// tenant_id component of a composite FK as an independent person edge
		// would connect EVERY record of that school to the subject.
		if rel.Column == "tenant_id" || rel.TargetColumn == "tenant_id" || seen[rel] {
			continue
		}
		seen[rel] = true
		records = append(records, rel)
	}
	return records
}

func personMetadataLinks(table string, fields map[string]json.RawMessage) []personKey {
	var metadata map[string]json.RawMessage
	if json.Unmarshal(fields["metadata"], &metadata) != nil {
		return nil
	}
	var resource string
	_ = json.Unmarshal(fields["resource_type"], &resource)
	var keys []personKey
	if table == "audit.data_access_log" {
		switch resource {
		case "staff_financial_view", "staff_financial_reveal", "staff_document_download":
			keys = appendPersonKey(keys, "users.staff_school_memberships", "id", metadata["staff_id"])
		case "guardian_financial_view", "guardian_financial_reveal":
			keys = appendPersonKey(keys, "users.guardian_profiles", "id", metadata["guardian_profile_id"])
		}
	}
	if table == "audit.auth_events" {
		var event string
		_ = json.Unmarshal(fields["event_type"], &event)
		if event == "staff_preview_started" || event == "staff_preview_ended" {
			keys = appendPersonKey(keys, "auth.accounts", "id", metadata["target_account_id"])
		}
	}
	return keys
}

// Context can bring in a conversation's contents, NOT another person's
// dependent records or every member of a group/request/folder. In particular
// enrollment.requests does not expand to the subject's siblings.
func personContentChild(parent, child string) bool {
	switch parent {
	case "users.parent_message_threads":
		return child == "users.parent_messages"
	case "users.staff_message_threads":
		return child == "users.staff_messages" || child == "users.staff_message_participants"
	case "users.parent_announcements":
		return child == "users.parent_announcement_options" || child == "users.parent_announcement_declaration_versions" || child == "documents.announcement_attachments"
	case "enrollment.requests":
		return child == "enrollment.change_requests"
	case "enrollment.change_requests":
		return child == "enrollment.change_request_messages"
	case "enrollment.request_children":
		return child == "enrollment.request_child_offering_selections" || child == "enrollment.care_offering_bookings"
	case "calendar.appointments":
		return child == "calendar.appointment_occurrence_overrides"
	}
	return false
}

type personReviewItem struct {
	Table    string   `json:"table,omitempty"`
	Row      int64    `json:"row,omitempty"`
	Artifact string   `json:"artifact,omitempty"`
	Rules    []string `json:"rules"`
}

func (r *exportRun) writePersonReview() error {
	items := r.personReviewItems()
	for _, artifact := range r.manifest.Artifacts {
		if strings.HasPrefix(artifact.Path, "objects/") {
			items = append(items, personReviewItem{Artifact: artifact.Path, Rules: []string{"original-content-and-group-photos", "third-party-review"}})
		}
	}
	return r.writeJSON("review.json", struct {
		Status            string             `json:"status"`
		UnassignedObjects int64              `json:"unassignedObjects"`
		RequiredChecks    []string           `json:"requiredChecks"`
		Items             []personReviewItem `json:"items"`
	}{
		Status: "internal-review-only-not-approved-for-handover", UnassignedObjects: r.person.unassignedObjects,
		RequiredChecks: []string{
			"school-order-identity-authority-and-subject-ids",
			"all-items-redacted-or-lawfully-released-by-school",
			"indirect-references-in-unselected-school-rows-and-unassigned-originals",
			"catalog-excluded-columns-and-tables-lawful-context-without-credentials",
			"bulk-access-audits-deleted-identities-and-unlinked-enrollment-snapshots",
			"external-systems-and-article-15-context-supplied-by-school",
			"separate-approved-delivery-copy-encrypted-transfer-and-working-copy-erasure",
		}, Items: items,
	})
}

func (r *exportRun) personReviewItems() []personReviewItem {
	var items []personReviewItem
	for _, model := range r.models {
		var row int64
		for _, node := range r.person.rows[model.Name] {
			if !node.selected {
				continue
			}
			row++
			items = append(items, personReviewItem{Table: model.Name, Row: row, Rules: personReviewRules(model, node)})
		}
	}
	return items
}

func personReviewRules(model organizationtenancy.ExportModelTable, node *personNode) []string {
	rules := []string{"third-party-review"}
	if !node.related {
		rules = append(rules, "context-not-subject")
	}
	if strings.HasPrefix(model.Name, "audit.") {
		rules = append(rules, "audit-actors-and-historical-values")
	}
	for _, col := range model.Columns {
		if col.Type == "text" || col.Type == "character varying" || col.Type == "jsonb" || col.Type == "json" {
			rules = append(rules, "free-text-snapshots-and-embedded-persons")
			break
		}
	}
	return rules
}
