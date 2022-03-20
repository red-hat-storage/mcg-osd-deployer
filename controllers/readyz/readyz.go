package readyz

import (
	"fmt"
	"net/http"
)

var isReady bool

func ReadinessChecker(_ *http.Request) error {
	if isReady {
		return nil
	}
	return fmt.Errorf("not ready")
}

func SetReadiness() {
	isReady = true
}

func UnsetReadiness() {
	isReady = false
}
