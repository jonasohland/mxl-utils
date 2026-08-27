package mxl

import (
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func GetFlowIDFromPath(path string) (string, bool) {
	path = filepath.Base(path)
	id, found := strings.CutSuffix(path, ".mxl-flow")
	if !found {
		return "", false
	}

	if len(id) != 36 {
		return "", false
	}

	_, err := uuid.Parse(id)
	return id, err == nil
}

func IsFlowDir(path string) bool {
	_, ok := GetFlowIDFromPath(path)
	return ok
}
