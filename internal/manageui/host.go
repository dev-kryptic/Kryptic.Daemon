package manageui

import (
	"strings"

	"github.com/dev-kryptic/daemon/internal/config"
)

const CloudName = "Kryptic Cloud"

func HostLabel(api string) string {
	if IsCloudAPI(api) {
		return CloudName
	}
	return strings.TrimSpace(api)
}

func IsCloudAPI(api string) bool {
	left, err := config.NormalizeAPI(api)
	if err != nil {
		left = strings.TrimRight(strings.TrimSpace(api), "/")
	}
	right, _ := config.NormalizeAPI(config.DefaultAPI)
	return strings.EqualFold(left, right)
}
