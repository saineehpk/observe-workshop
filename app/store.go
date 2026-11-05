package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrUnknownUser  = errors.New("user not found")
	ErrInsufficient = errors.New("insufficient balance")
	// ErrDuplicateKey mimics the error text a unique index returns.
	ErrDuplicateKey = errors.New("E11000 duplicate key error collection: wallet_statement index: hash_1")
)

type Account struct {
	Credit int64 `json:"credit"`
	Pocket int64 `json:"pocket"`
	Stack  int   `json:"stack"`
}

// Store is an in-memory stand-in for the database. It keeps a primary copy
// and a "secondary" copy of each stack value that only catches up after
// replicaLag, the way a lagging read replica does.
type Store struct {
	mu         sync.Mutex
	accounts   map[string]*Account
	ledgerHash map[string]bool
	secondary  map[string]int
	replicaLag time.Duration
}

func newStore(users int, replicaLag time.Duration) *Store {
	s := &Store{
		accounts:   map[string]*Account{},
		ledgerHash: map[string]bool{},
		secondary:  map[string]int{},
		replicaLag: replicaLag,
	}
	for i := 1; i <= users; i++ {
		u := fmt.Sprintf("player%03d", i)
		s.accounts[u] = &Account{Credit: 1000, Pocket: 500, Stack: 1}
		s.secondary[u] = 1
	}
	return s
}

func (s *Store) Get(user string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[user]
	if !ok {
		return Account{}, ErrUnknownUser
	}
	return *a, nil
}

func (s *Store) Debit(user string, amount int64) (before, after int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[user]
	if !ok {
		return 0, 0, ErrUnknownUser
	}
	if a.Credit < amount {
		return a.Credit, a.Credit, ErrInsufficient
	}
	before = a.Credit
	a.Credit -= amount
	return before, a.Credit, nil
}

// ExchangePocket writes a ledger row first. Its hash only includes the
// minute, so the same user moving the same amount twice in one minute hits
// the unique index even though both requests are legitimate.
func (s *Store) ExchangePocket(user string, amount int64, now time.Time) (before, after Account, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[user]
	if !ok {
		return Account{}, Account{}, ErrUnknownUser
	}
	before = *a
	if a.Pocket < amount {
		return before, before, ErrInsufficient
	}
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%d|POCKET|%s", user, amount, now.Format("2006-01-02T15:04"))))
	hash := hex.EncodeToString(sum[:])
	if s.ledgerHash[hash] {
		return before, before, ErrDuplicateKey
	}
	s.ledgerHash[hash] = true
	a.Pocket -= amount
	a.Credit += amount
	return before, *a, nil
}

// ClaimCheckin marks today's stack as claimed (-1) on the primary.
func (s *Store) ClaimCheckin(user string, reward int64) (before Account, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[user]
	if !ok {
		return Account{}, ErrUnknownUser
	}
	before = *a
	if a.Stack == -1 {
		return before, errAlreadyClaimed
	}
	a.Stack = -1
	a.Credit += reward
	s.replicate(user, -1)
	return before, nil
}

var errAlreadyClaimed = errors.New("already claimed today")

// ReadStackSecondary reads from the lagging copy.
func (s *Store) ReadStackSecondary(user string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.secondary[user]
}

// SetStack writes the primary and returns the value it replaced.
func (s *Store) SetStack(user string, v int) (previous int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[user]
	if !ok {
		return 0
	}
	previous = a.Stack
	a.Stack = v
	s.replicate(user, v)
	return previous
}

// replicate must be called with s.mu held.
func (s *Store) replicate(user string, v int) {
	time.AfterFunc(s.replicaLag, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.secondary[user] = v
	})
}
