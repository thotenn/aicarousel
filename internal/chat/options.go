package chat

// Options carries the per-request sampling parameters sent by the caller.
// Pointer fields distinguish "not set" from a deliberate zero value: unset
// fields keep the provider defaults from internal/providers/provparams.
type Options struct {
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Stop        *string
}

// Float64Ptr returns a pointer to v. Helper for building Options.
func Float64Ptr(v float64) *float64 { return &v }

// IntPtr returns a pointer to v. Helper for building Options.
func IntPtr(v int) *int { return &v }
