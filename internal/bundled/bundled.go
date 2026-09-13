package bundled

import "embed"

// Payloads are downloaded and verified by cmd/bundle-core before packaging.
//
//go:embed payload/*
var Files embed.FS
