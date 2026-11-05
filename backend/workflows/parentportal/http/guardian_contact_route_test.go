package parent_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/common"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parent "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// guardianContactCapture records what the create-contact route hands to the
// workflow. The embedded interfaces stay nil: these tests call no other method.
type guardianContactCapture struct {
	parent.PortalService
	parentService.RequestSharingService
	calls int
	got   parentService.CreateGuardianContactInput
}

func (c *guardianContactCapture) CreateGuardianContact(_ context.Context, _, _ int64, input parentService.CreateGuardianContactInput) (*parentService.ChildGuardian, error) {
	c.calls++
	c.got = input
	return &parentService.ChildGuardian{FirstName: input.Contact.FirstName, LastName: input.Contact.LastName}, nil
}

// realisticSignaturePNG draws a signature like the portal's canvas sends it
// from a high-density phone: a 1200x320 PNG with anti-aliased strokes. Its
// base64 form is tens of kilobytes, like the signatures seen in production
// (#5697).
func realisticSignaturePNG(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 320))
	rng := rand.New(rand.NewSource(5697))
	x, y := 60.0, 160.0
	for step := 0; step < 9000; step++ {
		x += rng.Float64()*1.4 - 0.2
		y += rng.Float64()*6 - 3
		if x > 1140 {
			x = 60
		}
		y = min(max(y, 40), 280)
		for dx := -3; dx <= 3; dx++ {
			for dy := -3; dy <= 3; dy++ {
				alpha := uint8(255 - (dx*dx+dy*dy)*12 - rng.Intn(40))
				img.SetNRGBA(int(x)+dx, int(y)+dy, color.NRGBA{A: alpha})
			}
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func postGuardianContact(t *testing.T, body []byte) (*httptest.ResponseRecorder, *guardianContactCapture) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	service := &guardianContactCapture{}
	router := testpkg.TenantRuntimeMiddleware(t, db)(
		parent.NewResource(parent.ResourceConfig{Parent: service}).Router(api.ParentSessionChain))

	req := httptest.NewRequest(http.MethodPost, "/me/children/"+strconv.FormatInt(chain.StudentID, 10)+"/guardians", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+parentToken(t, chain.AccountID))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec, service
}

// A new contact with a signed pickup permission passes the real route with a
// signature of realistic size; the former 8 KB limit cut it off (#5697).
func TestCreateGuardianContactRoute_AcceptsRealisticSignature(t *testing.T) {
	t.Parallel()

	signature := realisticSignaturePNG(t)
	require.Greater(t, len(signature), 64<<10, "the signature must be as large as real ones")
	body, err := json.Marshal(map[string]any{
		"first_name": "Erika", "last_name": "Muster", "relationship_type": "relative",
		"can_pickup": true, "pickup_weekdays": []int{2},
		"pickup_signature": map[string]string{"method": "drawn", "image_png": signature, "statement": "Ich erlaube …"},
	})
	require.NoError(t, err)

	rec, service := postGuardianContact(t, body)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, 1, service.calls)
	require.NotNil(t, service.got.PickupSignature)
	assert.Equal(t, signature, service.got.PickupSignature.ImagePNG, "the signature reaches the workflow unchanged")
	assert.Equal(t, "Ich erlaube …", service.got.PickupSignature.Statement)
}

// A body beyond the signature limit gets its own code instead of the generic
// input error, and nothing is created.
func TestCreateGuardianContactRoute_RejectsOversizedBodyWithOwnCode(t *testing.T) {
	t.Parallel()

	body := []byte(`{"first_name":"Erika","last_name":"Muster","pickup_signature":{"method":"drawn","image_png":"` +
		strings.Repeat("A", 400<<10) + `"}}`)

	rec, service := postGuardianContact(t, body)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
	var problem struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	assert.Equal(t, common.CodeCarePickupSignatureTooLarge, problem.Code)
	assert.Zero(t, service.calls)
}
