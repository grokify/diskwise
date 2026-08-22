package policy

import (
	"testing"

	"github.com/grokify/diskwise/entity"
)

func TestIsManagedData(t *testing.T) {
	managed := []entity.Kind{entity.KindDatabase, entity.KindContainer, entity.KindVM, entity.KindApplication}
	for _, k := range managed {
		if !IsManagedData(k) {
			t.Errorf("IsManagedData(%s) = false, want true", k)
		}
	}
	unmanaged := []entity.Kind{entity.KindCache, entity.KindArchive, entity.KindUnknown}
	for _, k := range unmanaged {
		if IsManagedData(k) {
			t.Errorf("IsManagedData(%s) = true, want false", k)
		}
	}
}

func TestEvaluate_ManagedDataGuard(t *testing.T) {
	cases := []struct {
		name     string
		proposed ActionClass
		kind     entity.Kind
		want     ActionClass
	}{
		{"database can't be safe_delete even at full confidence", SafeDelete, entity.KindDatabase, BackupThenDelete},
		{"container can't be likely_safe", LikelySafe, entity.KindContainer, BackupThenDelete},
		{"vm backup_then_delete passes through unchanged", BackupThenDelete, entity.KindVM, BackupThenDelete},
		{"application keep passes through unchanged (already conservative)", Keep, entity.KindApplication, Keep},
		{"application review passes through unchanged", Review, entity.KindApplication, Review},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Evaluate(c.proposed, c.kind, 1.0)
			if got != c.want {
				t.Errorf("Evaluate(%s, %s, 1.0) = %s, want %s", c.proposed, c.kind, got, c.want)
			}
		})
	}
}

func TestEvaluate_ConfidenceGuard(t *testing.T) {
	got := Evaluate(SafeDelete, entity.KindCache, 0.5)
	if got != LikelySafe {
		t.Errorf("low-confidence SafeDelete = %s, want downgrade to LikelySafe", got)
	}

	got = Evaluate(SafeDelete, entity.KindCache, MinConfidenceForSafeDelete)
	if got != SafeDelete {
		t.Errorf("confidence exactly at threshold = %s, want SafeDelete to survive", got)
	}

	got = Evaluate(LikelySafe, entity.KindCache, 0.1)
	if got != LikelySafe {
		t.Errorf("low confidence should not affect a non-SafeDelete proposal, got %s", got)
	}
}

func TestEvaluate_UnmanagedHighConfidencePassesThrough(t *testing.T) {
	got := Evaluate(SafeDelete, entity.KindCache, 1.0)
	if got != SafeDelete {
		t.Errorf("got %s, want SafeDelete", got)
	}
}
