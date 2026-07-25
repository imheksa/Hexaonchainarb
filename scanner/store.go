package scanner

import (
	"sync"
	"time"
)

const maxStoredOpportunities = 500

// PriceStore is a thread-safe in-memory store for prices and opportunities.
type PriceStore struct {
	mu      sync.RWMutex
	prices  map[PriceKey]*PriceData
	opps    []*Opportunity
	maxAge  time.Duration
}

func NewPriceStore(maxAgeSec int) *PriceStore {
	return &PriceStore{
		prices: make(map[PriceKey]*PriceData),
		maxAge: time.Duration(maxAgeSec) * time.Second,
	}
}

func (s *PriceStore) SetPrice(key PriceKey, data *PriceData) {
	s.mu.Lock()
	s.prices[key] = data
	s.mu.Unlock()
}

func (s *PriceStore) GetPrice(key PriceKey) (*PriceData, bool) {
	s.mu.RLock()
	p, ok := s.prices[key]
	s.mu.RUnlock()
	return p, ok
}

// AllPrices returns all prices, excluding stale entries.
func (s *PriceStore) AllPrices() []*PriceData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*PriceData, 0, len(s.prices))
	for _, p := range s.prices {
		if time.Since(p.UpdatedAt) <= s.maxAge {
			out = append(out, p)
		}
	}
	return out
}

// PricesBySymbol returns fresh prices for a given token pair symbol.
func (s *PriceStore) PricesBySymbol(symbol string) []*PriceData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*PriceData
	for _, p := range s.prices {
		if p.Symbol == symbol && time.Since(p.UpdatedAt) <= s.maxAge {
			out = append(out, p)
		}
	}
	return out
}

// UniqueSymbols returns all symbols that have at least one fresh price.
func (s *PriceStore) UniqueSymbols() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, p := range s.prices {
		if time.Since(p.UpdatedAt) <= s.maxAge {
			seen[p.Symbol] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for sym := range seen {
		out = append(out, sym)
	}
	return out
}

func (s *PriceStore) AddOpportunity(opp *Opportunity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opps = append(s.opps, opp)
	if len(s.opps) > maxStoredOpportunities {
		s.opps = s.opps[len(s.opps)-maxStoredOpportunities:]
	}
}

// RecentOpportunities returns the last `limit` opportunities (newest last).
func (s *PriceStore) RecentOpportunities(limit int) []*Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.opps)
	if limit > n {
		limit = n
	}
	out := make([]*Opportunity, limit)
	copy(out, s.opps[n-limit:])
	return out
}

func (s *PriceStore) TotalOpportunities() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.opps)
}
