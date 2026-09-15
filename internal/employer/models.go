package employer

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type CreateEmployerRequest struct {
	Name             string   `json:"name" validate:"required"`
	Industry         string   `json:"industry" validate:"required"`
	Location         string   `json:"location" validate:"required"`
	HiringModalities []string `json:"hiring_modalities" validate:"dive,required"`
}

func (r *CreateEmployerRequest) UnmarshalJSON(data []byte) error {
	var payload struct {
		Name             string          `json:"name"`
		Industry         string          `json:"industry"`
		Location         string          `json:"location"`
		HiringModalities json.RawMessage `json:"hiring_modalities"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(payload.HiringModalities), []byte("null")) {
		return errors.New("hiring_modalities must be an array")
	}

	r.Name = payload.Name
	r.Industry = payload.Industry
	r.Location = payload.Location
	r.HiringModalities = nil
	if len(payload.HiringModalities) > 0 {
		if err := json.Unmarshal(payload.HiringModalities, &r.HiringModalities); err != nil {
			return err
		}
	}
	return nil
}

func (r *CreateEmployerRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Industry = strings.TrimSpace(r.Industry)
	r.Location = strings.TrimSpace(r.Location)

	if r.HiringModalities == nil {
		r.HiringModalities = []string{}
	}
	for i := range r.HiringModalities {
		r.HiringModalities[i] = strings.TrimSpace(r.HiringModalities[i])
	}
}

type Employer struct {
	ID               int32     `json:"id"`
	UserID           int32     `json:"user_id"`
	Name             string    `json:"name"`
	Industry         string    `json:"industry"`
	Location         string    `json:"location"`
	HiringModalities []string  `json:"hiring_modalities"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (e *Employer) Normalize() {
	if e.HiringModalities == nil {
		e.HiringModalities = []string{}
	}
}
