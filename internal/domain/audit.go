package domain

import (
	"fmt"
	"time"
)

func (d *InterviewDossier) AppendAudit(action, actor string, now time.Time) {
	previous := ""
	if len(d.Audit) > 0 {
		previous = d.Audit[len(d.Audit)-1].SHA256
	}
	sequence := len(d.Audit) + 1
	payload := fmt.Sprintf("%s|%d|%s|%s|%s", d.DossierID, sequence, action, actor, previous)
	d.Audit = append(d.Audit, AuditEvent{Sequence: sequence, Action: action, ActorID: actor, At: now.UTC(), PreviousSHA256: previous, SHA256: Digest(payload)})
}

func VerifyAudit(d *InterviewDossier) error {
	previous := ""
	for i, event := range d.Audit {
		if event.Sequence != i+1 || event.PreviousSHA256 != previous {
			return ErrValidation
		}
		payload := fmt.Sprintf("%s|%d|%s|%s|%s", d.DossierID, event.Sequence, event.Action, event.ActorID, previous)
		if event.SHA256 != Digest(payload) {
			return ErrValidation
		}
		previous = event.SHA256
	}
	return nil
}
