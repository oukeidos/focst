package gemini

import (
	"errors"
	"strings"
	"testing"

	"github.com/oukeidos/focst/internal/apperrors"
	"google.golang.org/api/googleapi"
)

func TestClassifyGeminiError_CodeMapping(t *testing.T) {
	t.Run("auth errors are non-retryable", func(t *testing.T) {
		err := classifyGeminiError(&googleapi.Error{Code: 401})
		assertErrorKind(t, err, apperrors.KindAuth)
		if apperrors.IsRetryable(err) {
			t.Fatalf("expected non-retryable error for 401")
		}
	})

	t.Run("bad request errors are non-retryable", func(t *testing.T) {
		err := classifyGeminiError(&googleapi.Error{Code: 400})
		assertErrorKind(t, err, apperrors.KindBadRequest)
		if apperrors.IsRetryable(err) {
			t.Fatalf("expected non-retryable error for 400")
		}
	})

	t.Run("not found errors are non-retryable", func(t *testing.T) {
		err := classifyGeminiError(&googleapi.Error{Code: 404})
		assertErrorKind(t, err, apperrors.KindBadRequest)
		if apperrors.IsRetryable(err) {
			t.Fatalf("expected non-retryable error for 404")
		}
	})

	t.Run("rate limit is retryable", func(t *testing.T) {
		err := classifyGeminiError(&googleapi.Error{Code: 429})
		assertErrorKind(t, err, apperrors.KindRateLimit)
		if !apperrors.IsRetryable(err) {
			t.Fatalf("expected retryable error for 429")
		}
	})

	t.Run("server errors are retryable", func(t *testing.T) {
		err := classifyGeminiError(&googleapi.Error{Code: 503})
		assertErrorKind(t, err, apperrors.KindTransient)
		if !apperrors.IsRetryable(err) {
			t.Fatalf("expected retryable error for 503")
		}
	})
}

func TestClassifyGeminiError_Unknown(t *testing.T) {
	err := classifyGeminiError(errors.New("boom"))
	assertErrorKind(t, err, apperrors.KindTransient)
	if !apperrors.IsRetryable(err) {
		t.Fatalf("expected retryable error for unknown error")
	}
}

func TestClassifyGeminiError_DoesNotExposeRawMessage(t *testing.T) {
	err := classifyGeminiError(errors.New("SECRET_SUBTITLE_LINE"))
	if strings.Contains(err.Error(), "SECRET_SUBTITLE_LINE") {
		t.Fatalf("expected safe message, got %q", err.Error())
	}
}

func assertErrorKind(t *testing.T, err error, kind apperrors.Kind) {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperrors.Error, got %T", err)
	}
	if appErr.Kind != kind {
		t.Fatalf("expected kind %s, got %s", kind, appErr.Kind)
	}
}
