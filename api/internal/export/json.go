package export

import (
	"encoding/json"
	"io"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
)

type jsonEnvelope struct {
	Version  int                 `json:"version"`
	Estimate *estimates.Estimate `json:"estimate"`
}

func WriteJSON(w io.Writer, est *estimates.Estimate) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonEnvelope{Version: 1, Estimate: est})
}
