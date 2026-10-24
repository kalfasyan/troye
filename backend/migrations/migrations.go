// Package migrations defines the database schema as code. Each file adds one
// migration with m.Register; PocketBase applies new ones when the app starts.
//
// Create one with: go run . migrate create <name>
package migrations
