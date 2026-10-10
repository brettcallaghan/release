// Package pluginscripts holds the integration tests for the scripts the
// release plugin's skills bundle under plugin/skills/*/scripts. It has no
// code of its own: each test runs a script with bash against an httptest
// server standing in for the GitHub API.
package pluginscripts
