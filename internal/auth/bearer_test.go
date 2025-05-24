package auth

import (
	"net/http"
	"testing"
)

func TestGetBearerToken(t *testing.T) {
	bearer1 := "Bearer testtoken"
	bearer2 := ""
	bearer3 := "this is not formatted right"

	bearers := []struct {
		Test      string
		Bearer    string
		WantToken string
		WantError bool
	}{
		{
			Test:      "Test 1",
			Bearer:    bearer1,
			WantToken: "testtoken",
			WantError: false,
		}, {
			Test:      "Test 2",
			Bearer:    bearer2,
			WantToken: "",
			WantError: true,
		}, {
			Test:      "Test 3",
			Bearer:    bearer3,
			WantToken: "",
			WantError: true,
		},
	}

	for _, tt := range bearers {
		t.Run(tt.Test, func(t *testing.T) {
			headers := make(http.Header)
			headers.Set("Authorization", tt.Bearer)
			getBearer, err := GetBearerToken(headers)
			// First check: Does the error state match expectation?
			if (err != nil) != tt.WantError {
				t.Errorf("error state mismatch")
				return
			}

			// Second check: Does the token match expectation?
			// (Only do this if the first check passed)
			if getBearer != tt.WantToken {
				t.Errorf("token mismatch, Bearer: %s, WantToken: %s", getBearer, tt.WantToken)
				return
			}
		})
	}
}
