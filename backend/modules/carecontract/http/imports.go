package carecontracthttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// The import of existing contracts in the Träger-Portal (#4723). The
// preview and the import take the same multipart form: the file, the
// mapping as JSON and, for the import, the fingerprint of the confirmed
// preview. The owner opens its units itself: each school's tenant
// transaction, the Träger transaction and the access log of the schools, so
// the routes carry no transaction.

// importFormOverhead bounds the form fields beside the file.
const importFormOverhead = 256 << 10

// importExtensions are the file names the import reads.
var importExtensions = []string{".csv", ".txt"}

// errImportFileType reports a file that is no CSV by its name.
var errImportFileType = errors.New("the import file must be a .csv or .txt file")

// ImportResource serves /traeger/contract-imports.
type ImportResource struct {
	imports         carecontract.ContractImports
	requireContract common.Middleware
}

func NewImportResource(imports carecontract.ContractImports, requireContract common.Middleware) *ImportResource {
	if imports == nil || requireContract == nil {
		panic("care contract http: the imports and the right check are required")
	}
	return &ImportResource{imports: imports, requireContract: requireContract}
}

// Router returns the routes mounted at /traeger/contract-imports.
func (rs *ImportResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		open := r.With(rs.requireContract)
		open.Get("/", rs.log)
		open.Post("/", rs.apply)
		open.Post("/preview", rs.preview)
	})
	return r
}

func (rs *ImportResource) preview(w http.ResponseWriter, r *http.Request) { rs.look(w, r, false) }

func (rs *ImportResource) apply(w http.ResponseWriter, r *http.Request) { rs.look(w, r, true) }

func (rs *ImportResource) look(w http.ResponseWriter, r *http.Request, apply bool) {
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
	result, err := rs.imports.ImportContracts(r.Context(), reviewer, request)
	if err != nil {
		renderImportError(w, r, err)
		return
	}
	status := http.StatusOK
	if apply {
		status = http.StatusCreated
	}
	common.Respond(w, r, status, result, "Contract import read")
}

func (rs *ImportResource) log(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	entries, err := rs.imports.ContractImportLog(r.Context(), reviewer)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, entries, "Contract imports retrieved")
}

// importRequestOf reads the upload: a file of at most
// MaxContractImportFileBytes named .csv or .txt, and the mapping. The owner
// checks the content: text with a header row, never a binary file.
func importRequestOf(w http.ResponseWriter, r *http.Request) (carecontract.ContractImportRequest, error) {
	limit := int64(carecontract.MaxContractImportFileBytes + importFormOverhead)
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseMultipartForm(limit); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return carecontract.ContractImportRequest{}, carecontract.ErrContractImportFileTooLarge
		}
		return carecontract.ContractImportRequest{}, carecontract.ErrContractImportFileInvalid
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	file, header, err := r.FormFile("file")
	if err != nil {
		return carecontract.ContractImportRequest{}, carecontract.ErrContractImportFileInvalid
	}
	defer func() { _ = file.Close() }()
	extension := strings.ToLower(filepath.Ext(header.Filename))
	if extension != importExtensions[0] && extension != importExtensions[1] {
		return carecontract.ContractImportRequest{}, errImportFileType
	}
	content, err := io.ReadAll(io.LimitReader(file, carecontract.MaxContractImportFileBytes+1))
	if err != nil {
		return carecontract.ContractImportRequest{}, carecontract.ErrContractImportFileInvalid
	}
	request := carecontract.ContractImportRequest{Content: content, FileName: header.Filename,
		Delimiter: r.FormValue("delimiter"), Fingerprint: r.FormValue("fingerprint")}
	if raw := r.FormValue("mapping"); raw != "" {
		var mapping carecontract.ContractImportMapping
		if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
			return carecontract.ContractImportRequest{}, &carecontract.ValidationError{Fields: map[string]string{"mapping": "invalid"}}
		}
		request.Mapping = &mapping
	}
	return request, nil
}

var importErrorRules = []struct {
	err    error
	render func(error) render.Renderer
}{
	{carecontract.ErrContractImportFileTooLarge, func(err error) render.Renderer {
		return common.ErrorRequestTooLargeWithCode(err, common.CodeCareContractImportFileTooLarge)
	}},
	{carecontract.ErrContractImportFileInvalid, func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeCareContractImportFileInvalid)
	}},
	{errImportFileType, func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeCareContractImportFileInvalid)
	}},
	{carecontract.ErrContractImportChanged, func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeCareContractImportChanged)
	}},
}

func renderImportError(w http.ResponseWriter, r *http.Request, err error) {
	for _, rule := range importErrorRules {
		if errors.Is(err, rule.err) {
			common.RenderError(w, r, rule.render(err))
			return
		}
	}
	renderError(w, r, err)
}
