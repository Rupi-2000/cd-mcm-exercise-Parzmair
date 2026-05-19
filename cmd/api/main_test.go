package main

import "testing"

func TestGetEnvReturnsFallback(t *testing.T) {
	t.Setenv("CATALOG_TEST_EMPTY", "")

	got := getEnv("CATALOG_TEST_EMPTY", "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestGetEnvReturnsValue(t *testing.T) {
	t.Setenv("CATALOG_TEST_VALUE", "configured")

	got := getEnv("CATALOG_TEST_VALUE", "fallback")
	if got != "configured" {
		t.Fatalf("expected configured value, got %q", got)
	}
}
