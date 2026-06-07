package internal

import (
	"net/http"
	"strconv"
)

func GetPathValueAsInt(r *http.Request, key string) (int32, error) {
	pathValue := r.PathValue(key)
	pathValueInt, err := strconv.ParseInt(pathValue, 10, 32)
	if err != nil {
		return 0, err
	}

	return int32(pathValueInt), nil
}
