package router

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gin-gonic/gin"
)

func TestRelayRouterRegistersPlaygroundVideoContentRoute(t *testing.T) {
	engine := gin.New()
	SetRelayRouter(engine)

	routes := engine.Routes()
	path := "/pg/videos/:task_id/content"
	methods := make(map[string]bool)
	for _, route := range routes {
		if route.Path == path {
			methods[route.Method] = true
		}
	}

	require.True(t, methods[http.MethodGet])
	require.True(t, methods[http.MethodHead])
}

func TestRelayRouterRegistersPlaygroundAssetUploadRoute(t *testing.T) {
	engine := gin.New()
	SetRelayRouter(engine)

	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/pg/assets" {
			return
		}
	}

	require.Fail(t, "POST /pg/assets route is not registered")
}
