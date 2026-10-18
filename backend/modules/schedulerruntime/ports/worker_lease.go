package ports

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// WorkerLeaseName names the one lease every Worker process competes for
// (#2726).
const WorkerLeaseName = "worker"

// The Worker lease timing. A crashed leader is replaced after at most
// TTL + retry (35 s), a stopped one after at most the retry interval (5 s).
const (
	workerLeaseTTL         = 30 * time.Second
	workerLeaseRenewEvery  = 10 * time.Second
	workerLeaseRetryEvery  = 5 * time.Second
	workerLeaseFenceMargin = 5 * time.Second
)

// NewWorkerLease configures the Worker's leadership over store with the
// lease timing above. Every call gets a holder ID of its own, so a restarted
// process never resumes the term of its predecessor. The root binds the
// store and the evidence.
func NewWorkerLease(store LeaseStore, evidence LeaseEvidence) (WorkerLease, error) {
	holder, err := workerHolderID()
	if err != nil {
		return WorkerLease{}, err
	}
	return WorkerLease{
		Store:       store,
		Name:        WorkerLeaseName,
		Holder:      holder,
		TTL:         workerLeaseTTL,
		RenewEvery:  workerLeaseRenewEvery,
		RetryEvery:  workerLeaseRetryEvery,
		FenceMargin: workerLeaseFenceMargin,
		Evidence:    evidence,
	}, nil
}

// workerHolders numbers the leases one process composes.
var workerHolders atomic.Int64

// workerHolderID names one Worker: host, process ID, and start time with a
// sequence number. A restarted process with a reused PID still differs in
// its start time.
func workerHolderID() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("worker lease holder: read hostname: %w", err)
	}
	started := strconv.FormatInt(time.Now().UnixNano(), 36)
	return fmt.Sprintf("%s/%d/%s-%d", host, os.Getpid(), started, workerHolders.Add(1)), nil
}
