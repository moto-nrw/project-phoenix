package compose

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// careEndPhotoRuntime records what the photo care end hands the runtime:
// the durable queue entries inside the transaction, the unlinks and
// broadcasts after the commit, and any consent trail entry.
type careEndPhotoRuntime struct {
	mu         sync.Mutex
	queued     []string
	unlinked   []string
	broadcasts []int64
	consents   int
}

func (r *careEndPhotoRuntime) PhotoFeatureEnabled(context.Context) (bool, error) { return true, nil }
func (r *careEndPhotoRuntime) ActingAccountID(context.Context) int64             { return 0 }
func (r *careEndPhotoRuntime) QueueUnlinkStoredPhoto(_ context.Context, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queued = append(r.queued, url)
	return nil
}
func (r *careEndPhotoRuntime) UnlinkStoredPhoto(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unlinked = append(r.unlinked, url)
}
func (r *careEndPhotoRuntime) BroadcastPhotoChange(_, studentID int64, _ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcasts = append(r.broadcasts, studentID)
}
func (r *careEndPhotoRuntime) RecordPhotoConsent(context.Context, StudentPhotoConsentSnapshot, StudentPhotoConsentSnapshot, *int64, time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.consents++
	return nil
}

// careEndToday is the fixed school day of every run below.
var careEndToday = calendar.NewDate(2026, time.November, 11)

type careEndFixture struct {
	db      *bun.DB
	ctx     context.Context
	tenant  int64
	runtime *careEndPhotoRuntime
	end     peopledirectory.StudentPhotoCareEndRetention
	logged  []peopledirectory.StudentPhotoCareEnd
	failLog func(peopledirectory.StudentPhotoCareEnd) error
}

func newCareEndFixture(t *testing.T) *careEndFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	runtime := &careEndPhotoRuntime{}
	directory, err := New(Dependencies{
		DB: db, StudentDirectory: testpkg.StudentDirectory{}, Observe: func(Observation) {}, StudentPhotos: runtime,
	}, nil)
	require.NoError(t, err)
	end := directory.StudentPhotoCareEnd(slog.Default())
	return &careEndFixture{
		db: db, ctx: testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db), tenant: testpkg.Tenant(t),
		runtime: runtime, end: end,
	}
}

// run ends the photos of the fixture's school in one committed tenant
// transaction, the way the nightly cleanup does.
func (f *careEndFixture) run(t *testing.T) (peopledirectory.StudentPhotoCareEndResult, error) {
	t.Helper()
	var result peopledirectory.StudentPhotoCareEndResult
	err := testpkg.WithinTenantTransaction(f.ctx, f.db, f.tenant, func(txCtx context.Context, _ bun.Tx) error {
		var runErr error
		result, runErr = f.end.EndStudentPhotosAfterCare(txCtx, careEndToday, func(_ context.Context, ended peopledirectory.StudentPhotoCareEnd) error {
			if f.failLog != nil {
				if err := f.failLog(ended); err != nil {
					return err
				}
			}
			f.logged = append(f.logged, ended)
			return nil
		})
		return runErr
	})
	return result, err
}

// child creates a child with a photo and a photo consent whose care ends on
// lastCareDay (nil: no recorded end) in status.
func (f *careEndFixture) child(t *testing.T, name string, lastCareDay *calendar.Date, status string) (int64, string) {
	t.Helper()
	student := testpkg.CreateTestStudent(t, f.db, name, "Foto", "2b")
	url := "/uploads/student-photos/" + name + ".jpg"
	_, err := f.db.NewRaw(`UPDATE users.student_profiles
		SET photo_path = ?, photo_consent_given_at = ?, photo_consent_given_by = NULL
		WHERE id = ?`, url, calendar.NewDate(2026, time.September, 1).BerlinMidnight(), student.ID).Exec(context.Background())
	require.NoError(t, err)
	_, err = f.db.NewRaw(`UPDATE users.student_school_memberships
		SET enrolled_from = ?, enrolled_until = ?, status = ?
		WHERE student_profile_id = ?`, calendar.NewDate(2025, time.August, 1), lastCareDay, status, student.ID).Exec(context.Background())
	require.NoError(t, err)
	return student.ID, url
}

type careEndPhotoRow struct {
	PhotoPath           *string    `bun:"photo_path"`
	PhotoConsentGivenAt *time.Time `bun:"photo_consent_given_at"`
	PhotoConsentGivenBy *int64     `bun:"photo_consent_given_by"`
}

func (f *careEndFixture) photo(t *testing.T, studentID int64) careEndPhotoRow {
	t.Helper()
	var row careEndPhotoRow
	require.NoError(t, f.db.NewRaw(`SELECT photo_path, photo_consent_given_at, photo_consent_given_by
		FROM users.student_profiles WHERE id = ?`, studentID).Scan(context.Background(), &row))
	return row
}

func day(d calendar.Date) *calendar.Date { return &d }

// TestStudentPhotoCareEndRemovesPhotoDataOnceTheCareIsOver pins the rule of
// #6650: the photo, its consent and who recorded it go in the first run
// after the last care day or the graduation, and only then. A last care
// day of today or later, or a child without a recorded end, keeps them.
// The file goes only after the commit, the consent trail gets no
// withdrawal, and a second run changes nothing.
func TestStudentPhotoCareEndRemovesPhotoDataOnceTheCareIsOver(t *testing.T) {
	t.Parallel()
	f := newCareEndFixture(t)

	var foreign int64
	t.Run("other school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign, _ = f.child(t, "Fremd", day(careEndToday.AddDays(-30)), "inactive")
	})
	ended, endedURL := f.child(t, "Beendet", day(careEndToday.AddDays(-1)), "inactive")
	graduate, graduateURL := f.child(t, "Abgang", nil, "alumnus")
	lastDayToday, _ := f.child(t, "Heute", day(careEndToday), "active")
	planned, _ := f.child(t, "Geplant", day(careEndToday.AddDays(40)), "active")
	open, _ := f.child(t, "Offen", nil, "active")

	result, err := f.run(t)
	require.NoError(t, err)
	assert.Equal(t, peopledirectory.StudentPhotoCareEndResult{Ended: 2}, result)

	for _, id := range []int64{ended, graduate} {
		assert.Equal(t, careEndPhotoRow{}, f.photo(t, id), "photo, consent and who recorded it go together")
	}
	for _, id := range []int64{lastDayToday, planned, open, foreign} {
		row := f.photo(t, id)
		assert.NotNil(t, row.PhotoPath, "a child still in care, or of another school, keeps its photo")
		assert.NotNil(t, row.PhotoConsentGivenAt)
	}
	assert.ElementsMatch(t, []string{endedURL, graduateURL}, f.runtime.queued, "the durable cleanup entries commit with the cleared rows")
	assert.ElementsMatch(t, []string{endedURL, graduateURL}, f.runtime.unlinked, "the files go after the commit")
	assert.ElementsMatch(t, []int64{ended, graduate}, f.runtime.broadcasts)
	assert.Zero(t, f.runtime.consents, "the end of care is no withdrawal by the parents")
	require.Len(t, f.logged, 2)
	byChild := map[int64]peopledirectory.StudentPhotoCareEnd{}
	for _, entry := range f.logged {
		byChild[entry.StudentID] = entry
	}
	assert.Equal(t, peopledirectory.StudentPhotoCareEnd{
		TenantID: f.tenant, StudentID: ended, LastCareDay: careEndToday.AddDays(-1).String(),
		PhotoRemoved: true, ConsentEnded: true,
	}, byChild[ended])
	assert.Equal(t, peopledirectory.StudentPhotoCareEnd{
		TenantID: f.tenant, StudentID: graduate, Graduated: true, PhotoRemoved: true, ConsentEnded: true,
	}, byChild[graduate])

	again, err := f.run(t)
	require.NoError(t, err)
	assert.Equal(t, peopledirectory.StudentPhotoCareEndResult{}, again, "a repeated run finds nothing left")
	assert.Len(t, f.logged, 2, "and logs nothing again")
}

// TestStudentPhotoCareEndDefersAHeldRowWithoutWaiting pins the lock rule of
// the nightly run: a child whose row another transaction holds is deferred
// unchanged at once, the other children still end, and the next run finds
// the deferred child in the database and ends it.
func TestStudentPhotoCareEndDefersAHeldRowWithoutWaiting(t *testing.T) {
	t.Parallel()
	f := newCareEndFixture(t)
	held, heldURL := f.child(t, "Gesperrt", day(careEndToday.AddDays(-2)), "inactive")
	free, freeURL := f.child(t, "Frei", day(careEndToday.AddDays(-2)), "inactive")

	holder, err := f.db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, err = holder.NewRaw(`SELECT id FROM users.student_profiles WHERE id = ? FOR UPDATE`, held).Exec(context.Background())
	require.NoError(t, err)

	started := time.Now()
	result, err := f.run(t)
	elapsed := time.Since(started)
	require.NoError(t, holder.Rollback())
	require.NoError(t, err)
	assert.Equal(t, peopledirectory.StudentPhotoCareEndResult{Ended: 1, Deferred: 1}, result)
	assert.Less(t, elapsed, 5*time.Second, "the run must not wait for the held row")
	assert.NotNil(t, f.photo(t, held).PhotoPath, "a deferred child keeps all of its photo data")
	assert.NotNil(t, f.photo(t, held).PhotoConsentGivenAt)
	assert.Equal(t, careEndPhotoRow{}, f.photo(t, free))
	assert.Equal(t, []string{freeURL}, f.runtime.unlinked)

	next, err := f.run(t)
	require.NoError(t, err)
	assert.Equal(t, peopledirectory.StudentPhotoCareEndResult{Ended: 1}, next, "the next run finds the deferred child again")
	assert.Equal(t, careEndPhotoRow{}, f.photo(t, held))
	assert.ElementsMatch(t, []string{freeURL, heldURL}, f.runtime.unlinked)
}

// TestStudentPhotoCareEndNeverLeavesAPartialEnd pins all or nothing: when
// the deletion log refuses a child, the run fails, its transaction rolls
// back, every child keeps its photo data, and no file is removed.
func TestStudentPhotoCareEndNeverLeavesAPartialEnd(t *testing.T) {
	t.Parallel()
	f := newCareEndFixture(t)
	first, _ := f.child(t, "Erstes", day(careEndToday.AddDays(-3)), "inactive")
	second, _ := f.child(t, "Zweites", day(careEndToday.AddDays(-3)), "inactive")
	refused := errors.New("deletion log refused")
	f.failLog = func(ended peopledirectory.StudentPhotoCareEnd) error {
		if ended.StudentID == second {
			return refused
		}
		return nil
	}

	_, err := f.run(t)
	require.ErrorIs(t, err, refused)
	for _, id := range []int64{first, second} {
		row := f.photo(t, id)
		assert.NotNil(t, row.PhotoPath, "a failed run removes no photo data")
		assert.NotNil(t, row.PhotoConsentGivenAt)
	}
	assert.Empty(t, f.runtime.unlinked, "no file goes before a commit")

	f.failLog = nil
	result, err := f.run(t)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Ended, "the next run ends both")
}

// TestStudentPhotoCareEndRefusesWithoutAPhotoRuntime keeps a composition
// without the root's photo runtime from clearing rows whose files no queue
// would remove.
func TestStudentPhotoCareEndRefusesWithoutAPhotoRuntime(t *testing.T) {
	t.Parallel()
	f := newCareEndFixture(t)
	child, _ := f.child(t, "Ohne", day(careEndToday.AddDays(-1)), "inactive")
	directory, err := New(Dependencies{DB: f.db, StudentDirectory: testpkg.StudentDirectory{}, Observe: func(Observation) {}}, nil)
	require.NoError(t, err)
	f.end = directory.StudentPhotoCareEnd(slog.Default())

	_, err = f.run(t)
	require.Error(t, err)
	assert.NotNil(t, f.photo(t, child).PhotoPath)
}
