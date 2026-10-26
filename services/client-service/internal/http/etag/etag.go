package etag

import (
	"strconv"
	"strings"
)

func ParseIfMatch(value string) (int64, error) {
	if len(value) < 3 || !strings.HasPrefix(value, `"`) || !strings.HasSuffix(value, `"`) {
		return 0, strconv.ErrSyntax
	}
	rawVersion := value[1 : len(value)-1]
	for _, digit := range rawVersion {
		if digit < '0' || digit > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	version, err := strconv.ParseInt(rawVersion, 10, 64)
	if err != nil || version <= 0 {
		return 0, strconv.ErrSyntax
	}
	return version, nil
}
