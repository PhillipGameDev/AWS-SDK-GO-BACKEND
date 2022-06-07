package useragent

import (
	"context"
	"testing"

	"github.com/hashicorp/aws-sdk-go-base/v2/internal/config"
)

func TestFromContext(t *testing.T) {
	testcases := map[string]struct {
		setup    func() context.Context
		expected string
	}{
		"empty": {
			setup: func() context.Context {
				return context.Background()
			},
			expected: "",
		},
		"UserAgentProducts": {
			setup: func() context.Context {
				return context.WithValue(context.Background(), ContextScopedUserAgent, config.UserAgentProducts{
					{
						Name:    "first",
						Version: "1.2.3",
					},
					{
						Name:    "second",
						Version: "1.0.2",
						Comment: "a comment",
					},
				})
			},
			expected: "first/1.2.3 second/1.0.2 (a comment)",
		},
		"[]UserAgentProduct": {
			setup: func() context.Context {
				return context.WithValue(context.Background(), ContextScopedUserAgent, []config.UserAgentProduct{
					{
						Name:    "first",
						Version: "1.2.3",
					},
					{
						Name:    "second",
						Version: "1.0.2",
						Comment: "a comment",
					},
				})
			},
			expected: "first/1.2.3 second/1.0.2 (a comment)",
		},
		"invalid type": {
			setup: func() context.Context {
				return context.WithValue(context.Background(), ContextScopedUserAgent, "invalid")
			},
			expected: "",
		},
	}

	for name, testcase := range testcases {
		t.Run(name, func(t *testing.T) {
			ctx := testcase.setup()

			v := FromContext(ctx)

			if v != testcase.expected {
				t.Errorf("expected %q, got %q", testcase.expected, v)
			}
		})
	}
}
