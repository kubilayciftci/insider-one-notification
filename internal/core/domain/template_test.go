package domain

import "testing"

func TestResolveTemplate(t *testing.T) {
	tests := []struct {
		name    string
		content string
		vars    map[string]string
		want    string
	}{
		{
			name:    "single variable",
			content: "Hello {{name}}, welcome!",
			vars:    map[string]string{"name": "Kubilay"},
			want:    "Hello Kubilay, welcome!",
		},
		{
			name:    "multiple variables",
			content: "Order {{order_id}} for {{name}} is {{status}}",
			vars:    map[string]string{"order_id": "ORD-123", "name": "Ali", "status": "confirmed"},
			want:    "Order ORD-123 for Ali is confirmed",
		},
		{
			name:    "no variables",
			content: "Plain message with no placeholders",
			vars:    map[string]string{"name": "ignored"},
			want:    "Plain message with no placeholders",
		},
		{
			name:    "nil vars",
			content: "Hello {{name}}",
			vars:    nil,
			want:    "Hello {{name}}",
		},
		{
			name:    "empty vars",
			content: "Hello {{name}}",
			vars:    map[string]string{},
			want:    "Hello {{name}}",
		},
		{
			name:    "repeated variable",
			content: "{{code}} is your code. Enter {{code}} now.",
			vars:    map[string]string{"code": "1234"},
			want:    "1234 is your code. Enter 1234 now.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveTemplate(tt.content, tt.vars)
			if got != tt.want {
				t.Errorf("ResolveTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}
