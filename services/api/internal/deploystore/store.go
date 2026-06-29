// Package deploystore is an in-memory store of deployments.
//
// Phase 0: deployments live in memory so the control-plane deploy path
// (antctl deploy → API → Deployment record) works end to end before the
// scheduler/Nomad/Firecracker execution half exists. A Postgres-backed store
// replaces this without changing the handler interface.
package deploystore

import (
	"sort"
	"sync"

	"github.com/threemates/antariksh/services/api/internal/domain"
)

// Store holds deployments keyed by service.
type Store struct {
	mu      sync.RWMutex
	byID    map[domain.DeploymentID]domain.Deployment
	bySvc   map[domain.ServiceID][]domain.DeploymentID
}

func New() *Store {
	return &Store{
		byID:  make(map[domain.DeploymentID]domain.Deployment),
		bySvc: make(map[domain.ServiceID][]domain.DeploymentID),
	}
}

// Create records a new deployment.
func (s *Store) Create(d domain.Deployment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[d.ID] = d
	s.bySvc[d.ServiceID] = append(s.bySvc[d.ServiceID], d.ID)
}

// ListByService returns a service's deployments, newest first.
func (s *Store) ListByService(svc domain.ServiceID) []domain.Deployment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.bySvc[svc]
	out := make([]domain.Deployment, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.byID[id])
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}
