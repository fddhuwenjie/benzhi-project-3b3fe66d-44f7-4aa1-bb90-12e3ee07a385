package repository

import (
	"encoding/json"
	"oralarchive/internal/domain"
)

func encodeDossier(d *domain.InterviewDossier) ([]byte, error) { return json.Marshal(d) }
func decodeDossier(payload []byte) (*domain.InterviewDossier, error) {
	var d domain.InterviewDossier
	if err := json.Unmarshal(payload, &d); err != nil {
		return nil, err
	}
	return &d, nil
}
