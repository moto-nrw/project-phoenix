package domain

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"
)

// RecoveryProof is the hash of the recovery proof a refresh request presented. The
// raw secret goes no further than NewRecoveryProof.
type RecoveryProof struct {
	hash []byte
}

// NewRecoveryProof hashes the presented proof; an empty one proves nothing.
func NewRecoveryProof(raw string) RecoveryProof {
	if raw == "" {
		return RecoveryProof{}
	}
	sum := sha256.Sum256([]byte(raw))
	return RecoveryProof{hash: sum[:]}
}

// Hash is the digest a rotation stores on the predecessor, nil without a
// proof.
func (p RecoveryProof) Hash() []byte {
	if p.hash == nil {
		return nil
	}
	return append([]byte(nil), p.hash...)
}

// Matches compares in constant time. A missing proof or a hand-off without a
// stored digest is not recoverable and stays a replay.
func (p RecoveryProof) Matches(expected []byte) bool {
	return len(expected) == sha256.Size && len(p.hash) == sha256.Size && subtle.ConstantTimeCompare(expected, p.hash) == 1
}

// HandoffLink is what the hand-off engine reads of a refresh session.
type HandoffLink struct {
	ID                   int64
	Token                string
	RotatedAt            *time.Time
	ReplacementTokenHash *string
	RecoveryProofHash    []byte
}

// HandoffChain follows the hand-offs of one portal's refresh sessions. Account and
// operator sessions differ only in their store, their lineage rule and their
// sentinels.
type HandoffChain[S any] struct {
	Link func(S) HandoffLink
	// Successor locks the session with the given raw token; it returns
	// NotFound when there is none.
	Successor func(ctx context.Context, token string) (S, error)
	// SameLineage holds when next continues current's family.
	SameLineage func(current, next S) bool
	Grace       time.Duration
	MaxHops     int
	// Invalid is the portal's refusal: the caller revokes the family and
	// rejects after the revocation committed.
	Invalid  error
	NotFound error
	// Portal names the portal in infrastructure errors.
	Portal string
}

// Resolve follows a bounded, validated successor chain from the presented
// session. A chain exists only when a rotation committed but its response may
// not have reached the client; outside the grace reuse is a replay. The
// request proves possession of the presented token; later hops are trusted
// only after their persisted links were validated. It returns the head of
// the chain and whether the head is not the presented session.
func (c HandoffChain[S]) Resolve(ctx context.Context, presented S, proof RecoveryProof, now time.Time) (S, bool, error) {
	current := presented
	proofValidated := false
	for hop := 0; hop < c.MaxHops; hop++ {
		link := c.Link(current)
		if link.RotatedAt == nil {
			return current, link.ID != c.Link(presented).ID, nil
		}
		if link.ReplacementTokenHash == nil || link.RotatedAt.After(now) || now.Sub(*link.RotatedAt) > c.Grace {
			return current, false, c.Invalid
		}
		if !proofValidated && !proof.Matches(link.RecoveryProofHash) {
			return current, false, c.Invalid
		}
		proofValidated = true
		next, err := c.successor(ctx, current, link)
		if err != nil {
			return current, false, err
		}
		current = next
	}
	return current, false, c.Invalid
}

// successor locks the session a rotated one handed off to. Only the hash of
// the successor is stored, so its raw value is recomputed from the token
// this hop was presented with (#4902); a hand-off recorded before #4902 named
// a random successor and is not recoverable.
func (c HandoffChain[S]) successor(ctx context.Context, current S, link HandoffLink) (S, error) {
	var zero S
	token := SuccessorRefreshToken(link.Token)
	if HashToken(token) != *link.ReplacementTokenHash {
		return zero, c.Invalid
	}
	next, err := c.Successor(ctx, token)
	if errors.Is(err, c.NotFound) {
		return zero, c.Invalid
	}
	if err != nil {
		return zero, fmt.Errorf("follow %s refresh-token handoff: %w", c.Portal, err)
	}
	if !c.SameLineage(current, next) {
		return zero, c.Invalid
	}
	return next, nil
}
