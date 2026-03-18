package parser

import "sync"

// firstSetCache caches the candidates emitted by collect-mode parse functions.
type firstSetCache struct {
	mu sync.RWMutex
	m  map[string]*CandidateSet
}

var globalFirstSets = &firstSetCache{
	m: make(map[string]*CandidateSet),
}

func (c *firstSetCache) get(key string) *CandidateSet {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.m[key]
}

func (c *firstSetCache) set(key string, cs *CandidateSet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cs
}

// cachedCollect checks the FIRST-set cache before running collect.
// If cached, merges the cached set into the parser's candidates.
// Returns true if cache hit.
func (p *Parser) cachedCollect(key string, fn func()) bool {
	if cached := globalFirstSets.get(key); cached != nil {
		for _, tok := range cached.Tokens {
			p.addTokenCandidate(tok)
		}
		for _, rule := range cached.Rules {
			p.addRuleCandidate(rule.Rule)
		}
		return true
	}
	// Take snapshot, run fn, compute diff, cache.
	before := p.candidates.snapshot()
	fn()
	diff := p.candidates.diff(before)
	if diff != nil {
		globalFirstSets.set(key, diff)
	}
	return false
}
