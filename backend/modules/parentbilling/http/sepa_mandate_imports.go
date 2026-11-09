package parentbillinghttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The import of existing SEPA mandates (#6198) at
// /traeger/sepa-mandates/imports: POST /preview reads a file and answers
// every row, POST / takes over the ready rows, GET / lists the imports.
// The file comes as multipart field "file" (.csv or .txt, at most
// capability.MaxImportFileBytes); the owner refuses content that is no
// text. "mapping" (JSON of capability.SEPAMandateImportMapping),
// "delimiter" and "sequences" (JSON, line to first or recurring) are
// optional fields. The file is read in memory and never stored; no answer
// and no error repeats an IBAN.

// importFormOverhead is the room the multipart envelope and the other
// fields of the form get beside the file: the mapping and an answer for
// each of up to MaxImportRows lines.
const importFormOverhead = 256 << 10

// importFileExtensions are the names a CSV export of an office program has.
var importFileExtensions = []string{".csv", ".txt"}

func (rs *SEPAMandateResource) previewImport(w http.ResponseWriter, r *http.Request) {
	rs.importMandates(w, r, false)
}

func (rs *SEPAMandateResource) applyImport(w http.ResponseWriter, r *http.Request) {
	rs.importMandates(w, r, true)
}

func (rs *SEPAMandateResource) importMandates(w http.ResponseWriter, r *http.Request, apply bool) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	request, err := importRequestOf(w, r)
	if err != nil {
		renderImportError(w, r, err)
		return
	}
	request.Apply = apply
	result, err := rs.mandates.ImportSEPAMandates(r.Context(), reviewer, request)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderImportError(w, r, err)
		return
	}
	status := http.StatusOK
	if apply {
		status = http.StatusCreated
	}
	common.Respond(w, r, status, result, "SEPA mandate import read successfully")
}

func (rs *SEPAMandateResource) importLog(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	entries, err := rs.mandates.SEPAMandateImportLog(r.Context(), reviewer)
	if err != nil {
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, entries, "SEPA mandate imports retrieved successfully")
}

// errImportFileType reports a file whose name is no CSV export.
var errImportFileType = errors.New("the import file must be a .csv or .txt file")

// importRequestOf reads the form: the file within the size limit and with
// a CSV name, and the optional mapping, delimiter and answers.
func importRequestOf(w http.ResponseWriter, r *http.Request) (capability.SEPAMandateImportRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, capability.MaxImportFileBytes+importFormOverhead)
	if err := r.ParseMultipartForm(capability.MaxImportFileBytes + importFormOverhead); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return capability.SEPAMandateImportRequest{}, capability.ErrImportFileTooLarge
		}
		return capability.SEPAMandateImportRequest{}, capability.ErrImportFileInvalid
	}
	// The limit keeps the whole form in memory; RemoveAll drops any part
	// the parser still spilled to disk, so no copy of the file stays.
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	file, header, err := r.FormFile("file")
	if err != nil {
		return capability.SEPAMandateImportRequest{}, capability.ErrImportFileInvalid
	}
	defer func() { _ = file.Close() }()
	if !containsExtension(strings.ToLower(filepath.Ext(header.Filename))) {
		return capability.SEPAMandateImportRequest{}, errImportFileType
	}
	content, err := io.ReadAll(io.LimitReader(file, capability.MaxImportFileBytes+1))
	if err != nil {
		return capability.SEPAMandateImportRequest{}, capability.ErrImportFileInvalid
	}
	if len(content) > capability.MaxImportFileBytes {
		return capability.SEPAMandateImportRequest{}, capability.ErrImportFileTooLarge
	}
	request := capability.SEPAMandateImportRequest{Content: content, FileName: header.Filename, Delimiter: r.FormValue("delimiter"),
		Fingerprint: r.FormValue("fingerprint")}
	if raw := r.FormValue("mapping"); raw != "" {
		mapping := &capability.SEPAMandateImportMapping{}
		if err := json.Unmarshal([]byte(raw), mapping); err != nil {
			return capability.SEPAMandateImportRequest{}, validation.Errors{"mapping": errors.New("is no mapping")}
		}
		request.Mapping = mapping
	}
	sequences, err := importSequencesOf(r.FormValue("sequences"))
	if err != nil {
		return capability.SEPAMandateImportRequest{}, err
	}
	request.Sequences = sequences
	return request, nil
}

func containsExtension(extension string) bool {
	for _, allowed := range importFileExtensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

// importSequencesOf reads the answers per line: {"4": "recurring"}.
func importSequencesOf(raw string) (map[int]string, error) {
	if raw == "" {
		return nil, nil
	}
	var byLine map[string]string
	if err := json.Unmarshal([]byte(raw), &byLine); err != nil {
		return nil, validation.Errors{"sequences": errors.New("is no answer per line")}
	}
	sequences := make(map[int]string, len(byLine))
	for key, value := range byLine {
		line, err := strconv.Atoi(key)
		if err != nil || line < 2 {
			return nil, validation.Errors{"sequences": errors.New("names no line of the file")}
		}
		sequences[line] = value
	}
	return sequences, nil
}

// renderImportError answers a refused import. No message repeats a value
// of the file.
func renderImportError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.Is(err, capability.ErrImportFileTooLarge):
		common.RenderError(w, r, common.ErrorRequestTooLargeWithCode(err, common.CodeParentBillingSepaImportFileTooLarge))
	case errors.Is(err, capability.ErrImportFileInvalid), errors.Is(err, errImportFileType):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingSepaImportFileInvalid))
	case errors.Is(err, capability.ErrImportChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSepaImportChanged))
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingSepaImportMappingInvalid))
	default:
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
	}
}
