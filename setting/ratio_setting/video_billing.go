package ratio_setting

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const (
	VideoBillingModePerRequest = "per_request"
	VideoBillingModePerSecond  = "per_second"
)

var videoBillingModeMap types.RWMap[string, string]

func VideoBillingMode2JSONString() string {
	return videoBillingModeMap.MarshalJSONString()
}

func UpdateVideoBillingModeByJSONString(jsonStr string) error {
	var modes map[string]string
	if err := common.UnmarshalJsonStr(jsonStr, &modes); err != nil {
		return err
	}
	for model, mode := range modes {
		if model == "" || mode != VideoBillingModePerSecond {
			return fmt.Errorf("video billing mode for %q must be %q", model, VideoBillingModePerSecond)
		}
	}
	return types.LoadFromJsonStringWithCallback(&videoBillingModeMap, jsonStr, InvalidateExposedDataCache)
}

func GetVideoBillingMode(model string) string {
	model = FormatMatchingModelName(model)
	if mode, ok := videoBillingModeMap.Get(model); ok && mode == VideoBillingModePerSecond {
		return mode
	}
	return VideoBillingModePerRequest
}
