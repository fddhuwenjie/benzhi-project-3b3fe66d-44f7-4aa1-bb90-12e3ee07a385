package application

import "sync"

type dossierLocks struct {
	mu    sync.Mutex
	items map[string]*lockRef
}
type lockRef struct {
	mu    sync.Mutex
	users int
}

func newDossierLocks() *dossierLocks { return &dossierLocks{items: map[string]*lockRef{}} }
func (l *dossierLocks) lock(id string) func() {
	l.mu.Lock()
	ref := l.items[id]
	if ref == nil {
		ref = &lockRef{}
		l.items[id] = ref
	}
	ref.users++
	l.mu.Unlock()
	ref.mu.Lock()
	return func() {
		ref.mu.Unlock()
		l.mu.Lock()
		ref.users--
		if ref.users == 0 {
			delete(l.items, id)
		}
		l.mu.Unlock()
	}
}
