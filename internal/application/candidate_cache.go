package application

import "sync"

type candidateDigestCache struct {
	mu     sync.RWMutex
	values map[string]string
}

func newCandidateDigestCache() *candidateDigestCache {
	return &candidateDigestCache{values: map[string]string{}}
}

func (c *candidateDigestCache) digest(dossierID string, calculate func() (string, error)) (string, error) {
	c.mu.RLock()
	digest, ok := c.values[dossierID]
	c.mu.RUnlock()
	if ok {
		return digest, nil
	}
	digest, err := calculate()
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.values[dossierID] = digest
	c.mu.Unlock()
	return digest, nil
}

// invalidate drops the cached candidate digest for a dossier so the next read
// recomputes it from the current candidate text. It must be called whenever a
// dossier is mutated, otherwise Get could surface a stale digest from a prior
// remediation round and cause confirmation digest mismatches.
func (c *candidateDigestCache) invalidate(dossierID string) {
	c.mu.Lock()
	delete(c.values, dossierID)
	c.mu.Unlock()
}
