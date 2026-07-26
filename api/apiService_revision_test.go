package api

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoadDataSuccessSerializesServerRevisionLosslessly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	data := make(map[string]interface{})
	setLoadDataRevision(data, math.MaxUint64)
	jsonObj(context, data, nil)

	var message struct {
		Success bool `json:"success"`
		Obj     struct {
			LastUpdate string `json:"lastUpdate"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &message); err != nil {
		t.Fatalf("api/load response is not valid JSON: %v", err)
	}
	if !message.Success {
		t.Fatal("api/load success response was marked unsuccessful")
	}
	if got, want := message.Obj.LastUpdate, "18446744073709551615"; got != want {
		t.Fatalf("lastUpdate = %q, want %q", got, want)
	}
}
