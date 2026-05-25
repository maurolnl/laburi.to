package employee

import (
	"context"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

func (h *EmployeeHandler) GetTimezones(w http.ResponseWriter, r *http.Request, _ int32) {
	timezones, err := h.service.GetTimezones(r.Context())
	if err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	internal.RespondWithJson(w, http.StatusOK, timezones)
}

func (s *employeeService) GetTimezones(ctx context.Context) ([]Timezone, error) {
	return s.repo.GetTimezones(ctx)
}
