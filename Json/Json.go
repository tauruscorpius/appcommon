package Json

import (
	"github.com/json-iterator/go"
)

var (
	// Marshal is exported by SCP/.../Json package.
	Marshal = jsoniter.Marshal
	// Unmarshal is exported by SCP/.../Json package.
	Unmarshal = jsoniter.Unmarshal
	// MarshalIndent is exported by SCP/.../Json package.
	MarshalIndent = jsoniter.MarshalIndent
	// NewDecoder is exported by SCP/.../Json package.
	NewDecoder = jsoniter.NewDecoder
	// NewEncoder is exported by SCP/.../Json package.
	NewEncoder = jsoniter.NewEncoder
)
