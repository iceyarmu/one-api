package helper

import (
	"encoding/json"
	"fmt"
	"one-api/relay/common"
	"strings"

	"github.com/gin-gonic/gin"
)

func ModelMappedHelper(c *gin.Context, info *common.RelayInfo) error {
	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]string)
		err := json.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}
		if modelMap[info.OriginModelName] != "" {
			upstreamModelName := modelMap[info.OriginModelName]

			// 提取Provider
			if idx := strings.Index(upstreamModelName, "@"); idx != -1 {
				suffix := upstreamModelName[idx+1:]
				upstreamModelName = upstreamModelName[:idx]
				info.ProviderOrder = strings.Split(suffix, ",")
			}

			info.UpstreamModelName = upstreamModelName
			info.IsModelMapped = true
		}
	}
	return nil
}
