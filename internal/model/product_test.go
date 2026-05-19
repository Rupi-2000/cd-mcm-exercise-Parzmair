package model

import "testing"

func TestValidateEmptyName(t *testing.T) {
	p := Product{Name: "", Price: 10.0}
	if p.Validate() {
		t.Error("expected validation to fail for empty name")
	}
}

func TestValidateNegativePrice(t *testing.T) {
	p := Product{Name: "Widget", Price: -5.0}
	if p.Validate() {
		t.Error("expected validation to fail for negative price")
	}
}

func TestValidateValidProduct(t *testing.T) {
	p := Product{Name: "Widget", Price: 9.99}
	if !p.Validate() {
		t.Error("expected validation to pass for valid product")
	}
}

func TestValidateAllowsZeroPrice(t *testing.T) {
	p := Product{Name: "Free Sample", Price: 0}
	if !p.Validate() {
		t.Error("expected validation to pass for zero price")
	}
}
