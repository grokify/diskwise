// Package policy classifies findings into reclaimability action tiers
// (safe_delete, likely_safe, backup_then_delete, review, keep, unknown)
// using a fail-closed model that keeps detection confidence and action
// risk as separate concerns.
package policy
