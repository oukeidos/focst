package gemini

import (
	"errors"
	"fmt"

	"github.com/oukeidos/focst/internal/apperrors"
	"google.golang.org/api/googleapi"
)

func classifyGeminiError(err error) error {
	if err == nil {
		return nil
	}

	wrapped := fmt.Errorf("gemini generate content failed: %w", err)

	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		switch gerr.Code {
		case 400, 404:
			if gerr.Code == 404 {
				return apperrors.New(apperrors.KindBadRequest, "Gemini model not found or no access (404).", wrapped)
			}
			return apperrors.New(apperrors.KindBadRequest, "Gemini request rejected (400).", wrapped)
		case 401, 403:
			return apperrors.New(apperrors.KindAuth, fmt.Sprintf("Gemini authentication/authorization failed (%d).", gerr.Code), wrapped)
		case 429:
			return apperrors.New(apperrors.KindRateLimit, "Gemini rate limit exceeded (429). Please try again later.", wrapped)
		case 500, 503, 504:
			return apperrors.New(apperrors.KindTransient, fmt.Sprintf("Gemini service temporary error (%d). Please retry.", gerr.Code), wrapped)
		default:
			if gerr.Code >= 500 {
				return apperrors.New(apperrors.KindTransient, fmt.Sprintf("Gemini service temporary error (%d). Please retry.", gerr.Code), wrapped)
			}
			return apperrors.New(apperrors.KindBadRequest, fmt.Sprintf("Gemini API error (%d).", gerr.Code), wrapped)
		}
	}

	// Non-HTTP transport/runtime failures (DNS, socket, timeout, etc.)
	// should be retried because they are usually transient.
	return apperrors.New(apperrors.KindTransient, "Gemini request failed due to a temporary network/runtime error.", wrapped)
}
