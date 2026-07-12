package proxy

import "testing"

func TestServiceFromMethod(t *testing.T) {
	service, ok := serviceFromMethod("/marketing.Marketing/GetDocument")
	if !ok || service != "marketing.Marketing" {
		t.Fatalf("serviceFromMethod() = %q, %v", service, ok)
	}
}

func TestServiceFromMethodRejectsInvalidPath(t *testing.T) {
	if _, ok := serviceFromMethod("marketing.Marketing/GetDocument"); ok {
		t.Fatal("serviceFromMethod() accepted an invalid path")
	}
}
